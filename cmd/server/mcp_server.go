package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/user/ai2mysql-mcp-server/pkg/config"
	"github.com/user/ai2mysql-mcp-server/pkg/db"
)

// MCPMessage 表示MCP的请求或响应消息
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError 表示MCP错误
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ToolParams 工具调用参数
type ToolParams struct {
	Name   string          `json:"name"`
	Params json.RawMessage `json:"params"`
}

// SQLParams SQL参数
type SQLParams struct {
	SQL string `json:"sql"`
}

// MCPContent 内容
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPToolResult 工具调用结果
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
}

// 实现完整的MCP服务器
type MCPServer struct {
	dbManager *db.DBManager
	config    *config.Config
	reader    *bufio.Reader
	writer    *bufio.Writer
}

// NewMCPServer 创建新的MCP服务器
func NewMCPServer(dbManager *db.DBManager, cfg *config.Config) *MCPServer {
	return &MCPServer{
		dbManager: dbManager,
		config:    cfg,
		reader:    bufio.NewReader(os.Stdin),
		writer:    bufio.NewWriter(os.Stdout),
	}
}

// Run 运行服务器
func (s *MCPServer) Run() error {
	// 无限循环处理请求
	for {
		// 读取一行数据（JSON-RPC 消息）
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil // 正常退出
			}
			return fmt.Errorf("读取请求失败: %v", err)
		}

		// 解析JSON-RPC消息
		var message MCPMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			s.sendError(nil, -32700, "解析JSON失败", err.Error())
			continue
		}

		// 处理消息
		s.handleMessage(message)
	}
}

// handleMessage 处理MCP消息
func (s *MCPServer) handleMessage(message MCPMessage) {
	// 判断消息类型和方法
	switch message.Method {
	case "mcp/initialize":
		// 初始化请求
		s.handleInitialize(message)
	case "mcp/callTool":
		// 工具调用请求
		s.handleCallTool(message)
	default:
		// 不支持的方法
		s.sendError(message.ID, -32601, "方法不支持", message.Method)
	}
}

// handleInitialize 处理初始化请求
func (s *MCPServer) handleInitialize(message MCPMessage) {
	// 构造初始化响应
	info := map[string]interface{}{
		"name":    "MySQL MCP Server",
		"version": "1.0.0",
	}

	// 构造工具信息
	tools := []map[string]interface{}{
		{
			"name":        "mcp_mysql_query",
			"description": "执行MySQL查询（只读，SELECT语句）",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sql": map[string]interface{}{
						"type":        "string",
						"description": "要执行的SQL查询语句",
					},
				},
				"required": []string{"sql"},
			},
		},
		{
			"name":        "mcp_mysql_execute",
			"description": "执行MySQL更新操作（INSERT/UPDATE/DELETE等非查询语句）",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sql": map[string]interface{}{
						"type":        "string",
						"description": "要执行的SQL语句",
					},
				},
				"required": []string{"sql"},
			},
		},
	}

	// 构造完整的capabilities
	capabilities := map[string]interface{}{
		"tools": tools,
	}

	// 发送初始化响应
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result: map[string]interface{}{
			"info":         info,
			"capabilities": capabilities,
		},
	}

	s.sendResponse(response)
}

// handleCallTool 处理工具调用请求
func (s *MCPServer) handleCallTool(message MCPMessage) {
	// 解析工具调用参数
	var params ToolParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.sendError(message.ID, -32602, "无效参数", err.Error())
		return
	}

	// 根据工具名称分发处理
	switch params.Name {
	case "mcp_mysql_query":
		s.handleMySQLQuery(message.ID, params.Params)
	case "mcp_mysql_execute":
		s.handleMySQLExecute(message.ID, params.Params)
	default:
		s.sendError(message.ID, -32601, "不支持的工具", params.Name)
	}
}

// handleMySQLQuery 处理MySQL查询
func (s *MCPServer) handleMySQLQuery(id interface{}, rawParams json.RawMessage) {
	// 检查查询权限
	if !s.config.Permission.AllowQuery {
		s.sendError(id, -32000, "查询操作未被允许", nil)
		return
	}

	// 解析SQL参数
	var sqlParams SQLParams
	if err := json.Unmarshal(rawParams, &sqlParams); err != nil {
		s.sendError(id, -32602, "无效SQL参数", err.Error())
		return
	}

	// 执行查询
	result, err := s.dbManager.Query("default", sqlParams.SQL)
	if err != nil {
		s.sendError(id, -32000, err.Error(), nil)
		return
	}

	// 构造响应结果
	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		s.sendError(id, -32000, "序列化结果失败", err.Error())
		return
	}

	// 发送成功响应
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: MCPToolResult{
			Content: []MCPContent{
				{
					Type: "text",
					Text: string(jsonResult),
				},
			},
		},
	}

	s.sendResponse(response)
}

// handleMySQLExecute 处理MySQL执行
func (s *MCPServer) handleMySQLExecute(id interface{}, rawParams json.RawMessage) {
	// 解析SQL参数
	var sqlParams SQLParams
	if err := json.Unmarshal(rawParams, &sqlParams); err != nil {
		s.sendError(id, -32602, "无效SQL参数", err.Error())
		return
	}

	// 执行操作
	result, err := s.dbManager.Execute("default", sqlParams.SQL)
	if err != nil {
		s.sendError(id, -32000, err.Error(), nil)
		return
	}

	// 构造响应结果
	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		s.sendError(id, -32000, "序列化结果失败", err.Error())
		return
	}

	// 发送成功响应
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: MCPToolResult{
			Content: []MCPContent{
				{
					Type: "text",
					Text: string(jsonResult),
				},
			},
		},
	}

	s.sendResponse(response)
}

// sendResponse 发送响应
func (s *MCPServer) sendResponse(response MCPMessage) {
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化响应失败: %v\n", err)
		return
	}

	// 发送响应
	s.writer.Write(jsonResponse)
	s.writer.Write([]byte("\n"))
	s.writer.Flush()
}

// sendError 发送错误响应
func (s *MCPServer) sendError(id interface{}, code int, message string, data interface{}) {
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	s.sendResponse(response)
}
