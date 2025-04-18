package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/blanplan-ai/ai2mysql-mcp-server/pkg/config"
	"github.com/blanplan-ai/ai2mysql-mcp-server/pkg/db"
)

// Logger 提供日志记录功能
type Logger struct {
	debugMode bool
	logFile   *os.File
	mu        sync.Mutex
	logger    *log.Logger
}

// NewLogger 创建新的日志记录器
func NewLogger(debugMode bool) (*Logger, error) {
	l := &Logger{
		debugMode: debugMode,
	}

	if debugMode {
		// 创建日志文件
		file, err := os.OpenFile("/tmp/ai2mysql.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("无法创建日志文件: %v", err)
		}
		l.logFile = file
		l.logger = log.New(file, "", log.LstdFlags)
		l.Infof("日志系统初始化成功，正在写入 /tmp/ai2mysql.log")
	} else {
		// 使用标准错误输出
		l.logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	return l, nil
}

// Close 关闭日志文件
func (l *Logger) Close() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}

// Infof 输出信息级别日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 生成日志消息
	msg := fmt.Sprintf(format, args...)

	// 在调试模式下写入日志文件
	if l.logger != nil {
		l.logger.Printf("[INFO] %s", msg)
	}

	// 同时总是输出到标准错误（控制台）
	if !l.debugMode || l.logFile == nil {
		fmt.Fprintf(os.Stderr, "[INFO] %s\n", msg)
	}
}

// Errorf 输出错误级别日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 生成日志消息
	msg := fmt.Sprintf(format, args...)

	// 在调试模式下写入日志文件
	if l.logger != nil {
		l.logger.Printf("[ERROR] %s", msg)
	}

	// 同时总是输出到标准错误（控制台）
	if !l.debugMode || l.logFile == nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %s\n", msg)
	}
}

// Debugf 输出调试级别日志（仅在调试模式下）
func (l *Logger) Debugf(format string, args ...interface{}) {
	if !l.debugMode {
		return // 非调试模式下不输出调试日志
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 生成日志消息
	msg := fmt.Sprintf(format, args...)

	// 写入日志文件
	if l.logger != nil {
		l.logger.Printf("[DEBUG] %s", msg)
	}
}

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

// ToolCallParams tools/call方法的参数格式
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
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
	logger    *Logger
}

// NewMCPServer 创建新的MCP服务器
func NewMCPServer(dbManager *db.DBManager, cfg *config.Config, debugMode bool) (*MCPServer, error) {
	logger, err := NewLogger(debugMode)
	if err != nil {
		return nil, err
	}

	return &MCPServer{
		dbManager: dbManager,
		config:    cfg,
		reader:    bufio.NewReader(os.Stdin),
		writer:    bufio.NewWriter(os.Stdout),
		logger:    logger,
	}, nil
}

// Run 运行服务器
func (s *MCPServer) Run() error {
	// 关闭日志
	defer s.logger.Close()

	// 打印启动信息
	s.logger.Infof("MySQL MCP 服务器已启动，等待客户端连接...")
	s.logger.Infof("服务器支持以下工具: mcp_mysql_query, mcp_mysql_execute")
	s.logger.Debugf("详细配置信息: %+v", s.config)

	// 添加协议版本信息
	protocolVersion := "2024-11-05"
	s.logger.Debugf("服务器协议版本: %v", protocolVersion)

	// 无限循环处理请求
	for {
		// 读取一行数据（JSON-RPC 消息）
		s.logger.Debugf("等待客户端请求...")
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				s.logger.Infof("客户端连接已关闭")
				return nil // 正常退出
			}
			s.logger.Errorf("读取请求失败: %v", err)
			return fmt.Errorf("读取请求失败: %v", err)
		}

		s.logger.Debugf("收到原始请求: %s", line)

		// 解析JSON-RPC消息
		var message MCPMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			s.logger.Errorf("解析JSON失败: %v", err)
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
	s.logger.Infof("处理消息, 方法: %s, ID: %v", message.Method, message.ID)

	switch message.Method {
	case "initialize":
		// 初始化请求
		s.handleInitialize(message)
	case "initialized":
		// 收到初始化确认通知
		s.logger.Infof("收到初始化确认通知")
		// 不需要回复，这是一个通知
	case "tools/list":
		// 工具列表请求
		s.handleToolsList(message)
	case "mcp/callTool":
		// 工具调用请求
		s.handleCallTool(message)
	case "tools/call":
		// 工具调用请求（新版MCP协议）
		s.handleCallTool(message)
	default:
		// 不支持的方法
		s.logger.Errorf("不支持的方法: %s", message.Method)
		s.sendError(message.ID, -32601, "方法不支持", message.Method)
	}
}

// handleInitialize 处理初始化请求
func (s *MCPServer) handleInitialize(message MCPMessage) {
	// 写入诊断日志
	s.logger.Debugf("收到初始化请求: %+v", message)

	// 添加协议版本信息 - 使用字符串格式
	protocolVersion := "2024-11-05"

	// 构造初始化响应
	serverInfo := map[string]interface{}{
		"name":    "MySQL MCP Server",
		"version": "1.0.0",
	}

	// 构造工具信息 - 使用对象而不是数组
	toolsObj := map[string]interface{}{
		"mcp_mysql_query": map[string]interface{}{
			"name":        "mcp_mysql_query",
			"description": "执行MySQL查询（只读，SELECT语句）",
			"type":        "function",
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
		"mcp_mysql_execute": map[string]interface{}{
			"name":        "mcp_mysql_execute",
			"description": "执行MySQL更新操作（INSERT/UPDATE/DELETE等非查询语句）",
			"type":        "function",
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
		"tools": toolsObj,
	}

	// 发送初始化响应
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result: map[string]interface{}{
			"protocolVersion": protocolVersion,
			"serverInfo":      serverInfo,
			"capabilities":    capabilities,
		},
	}

	// 写入诊断日志
	respJson, _ := json.Marshal(response)
	s.logger.Debugf("发送初始化响应: %s", string(respJson))

	s.sendResponse(response)
}

// handleToolsList 处理工具列表请求
func (s *MCPServer) handleToolsList(message MCPMessage) {
	s.logger.Debugf("收到工具列表请求: %+v", message)

	// 构造工具数组 - tools/list 需要返回数组格式
	toolsArray := []map[string]interface{}{
		{
			"name":        "mcp_mysql_query",
			"description": "执行MySQL查询（只读，SELECT语句）",
			"type":        "function",
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
			"type":        "function",
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

	// 发送响应 - 将工具数组放在名为tools的字段中
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result: map[string]interface{}{
			"tools": toolsArray,
		},
	}

	// 写入诊断日志
	respJson, _ := json.Marshal(response)
	s.logger.Debugf("发送工具列表响应: %s", string(respJson))

	s.sendResponse(response)
}

// handleCallTool 处理工具调用请求
func (s *MCPServer) handleCallTool(message MCPMessage) {
	var toolName string
	var toolParams json.RawMessage

	// 根据不同的方法解析参数
	if message.Method == "tools/call" {
		// 解析tools/call方法的参数格式
		var params ToolCallParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			s.logger.Errorf("解析工具参数失败: %v", err)
			s.sendError(message.ID, -32602, "无效参数", err.Error())
			return
		}
		toolName = params.Name
		toolParams = params.Arguments
		s.logger.Debugf("使用tools/call格式解析参数: name=%s, arguments=%s", toolName, string(toolParams))
	} else {
		// 解析mcp/callTool方法的参数格式
		var params ToolParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			s.logger.Errorf("解析工具参数失败: %v", err)
			s.sendError(message.ID, -32602, "无效参数", err.Error())
			return
		}
		toolName = params.Name
		toolParams = params.Params
		s.logger.Debugf("使用mcp/callTool格式解析参数: name=%s, params=%s", toolName, string(toolParams))
	}

	s.logger.Infof("调用工具: %s", toolName)
	s.logger.Debugf("工具参数: %s", string(toolParams))

	// 根据工具名称分发处理
	switch toolName {
	case "mcp_mysql_query":
		s.handleMySQLQuery(message.ID, toolParams)
	case "mcp_mysql_execute":
		s.handleMySQLExecute(message.ID, toolParams)
	default:
		s.logger.Errorf("不支持的工具: %s", toolName)
		s.sendError(message.ID, -32601, "不支持的工具", toolName)
	}
}

// handleMySQLQuery 处理MySQL查询
func (s *MCPServer) handleMySQLQuery(id interface{}, rawParams json.RawMessage) {
	// 检查查询权限
	if !s.config.Permission.AllowQuery {
		s.logger.Errorf("查询操作未被允许")
		s.sendError(id, -32000, "查询操作未被允许", nil)
		return
	}

	// 解析SQL参数
	var sqlParams SQLParams
	if err := json.Unmarshal(rawParams, &sqlParams); err != nil {
		s.logger.Errorf("解析SQL参数失败: %v", err)
		s.sendError(id, -32602, "无效SQL参数", err.Error())
		return
	}

	s.logger.Infof("执行SQL查询: %s", sqlParams.SQL)
	startTime := time.Now()

	// 执行查询
	result, err := s.dbManager.Query("default", sqlParams.SQL)
	if err != nil {
		s.logger.Errorf("执行查询失败: %v", err)
		s.sendError(id, -32000, err.Error(), nil)
		return
	}

	s.logger.Infof("查询执行完成，耗时: %v，结果行数: %d", time.Since(startTime), len(result.Rows))
	s.logger.Debugf("查询结果: %+v", result)

	// 构造响应结果
	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		s.logger.Errorf("序列化结果失败: %v", err)
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
		s.logger.Errorf("解析SQL参数失败: %v", err)
		s.sendError(id, -32602, "无效SQL参数", err.Error())
		return
	}

	s.logger.Infof("执行SQL操作: %s", sqlParams.SQL)
	startTime := time.Now()

	// 执行操作
	result, err := s.dbManager.Execute("default", sqlParams.SQL)
	if err != nil {
		s.logger.Errorf("执行操作失败: %v", err)
		s.sendError(id, -32000, err.Error(), nil)
		return
	}

	s.logger.Infof("操作执行完成，耗时: %v，影响行数: %d", time.Since(startTime), result.RowsAffected)
	s.logger.Debugf("操作结果: %+v", result)

	// 构造响应结果
	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		s.logger.Errorf("序列化结果失败: %v", err)
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
		s.logger.Errorf("序列化响应失败: %v", err)
		return
	}

	// 发送响应
	_, err = s.writer.Write(jsonResponse)
	if err != nil {
		s.logger.Errorf("写入响应失败: %v", err)
		return
	}

	_, err = s.writer.Write([]byte("\n"))
	if err != nil {
		s.logger.Errorf("写入换行符失败: %v", err)
		return
	}

	err = s.writer.Flush()
	if err != nil {
		s.logger.Errorf("刷新缓冲区失败: %v", err)
		return
	}

	s.logger.Debugf("成功发送响应: %s", string(jsonResponse))
}

// sendError 发送错误响应
func (s *MCPServer) sendError(id interface{}, code int, message string, data interface{}) {
	s.logger.Errorf("准备发送错误: code=%d, message=%s, data=%v", code, message, data)

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
