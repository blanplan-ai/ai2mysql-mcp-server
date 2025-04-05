package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/blanplan-ai/ai2mysql-mcp-server/pkg/config"
	"github.com/blanplan-ai/ai2mysql-mcp-server/pkg/db"
)

var (
	configPath string
	debugMode  bool
)

func init() {
	flag.StringVar(&configPath, "config", "config.json", "配置文件路径")
	flag.BoolVar(&debugMode, "debug", false, "启用调试模式，将日志输出到/tmp/ai2mysql.log")
}

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建数据库管理器
	dbManager, err := db.NewDBManager(cfg)
	if err != nil {
		log.Fatalf("初始化数据库管理器失败: %v", err)
	}
	defer dbManager.Close()

	// 如果配置文件不存在，创建默认配置
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		dir := filepath.Dir(configPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("创建配置目录失败: %v", err)
		} else {
			defaultCfg := config.DefaultConfig()
			if err := config.SaveConfig(defaultCfg, configPath); err != nil {
				log.Printf("保存默认配置失败: %v", err)
			} else {
				log.Printf("已创建默认配置文件: %s", configPath)
			}
		}
	}

	// 启动MCP服务器
	server, err := NewMCPServer(dbManager, cfg, debugMode)
	if err != nil {
		log.Fatalf("创建MCP服务器失败: %v", err)
	}

	log.Printf("MySQL MCP 服务器已启动，等待连接...")
	if debugMode {
		log.Printf("调试模式已启用，日志将输出到 /tmp/ai2mysql.log")
	}

	if err := server.Run(); err != nil {
		log.Fatalf("服务器运行失败: %v", err)
	}
}

// 简单的MCP服务器实现
type MySQLMCPServer struct {
	dbManager *db.DBManager
	config    *config.Config
}

// 创建新的MCP服务器
func NewMySQLMCPServer(dbManager *db.DBManager, cfg *config.Config) *MySQLMCPServer {
	return &MySQLMCPServer{
		dbManager: dbManager,
		config:    cfg,
	}
}

// 运行服务器
func (s *MySQLMCPServer) Run() error {
	// 这里实现简单的stdio读写循环
	fmt.Println("MySQL MCP Server v1.0.0")
	fmt.Println("使用 SELECT 查询数据，使用 INSERT/UPDATE/DELETE 修改数据")
	fmt.Println("输入 'exit' 退出服务器")

	// 读取标准输入，循环处理命令
	for {
		// 从标准输入读取SQL语句
		var sql string
		fmt.Print("> ")
		fmt.Scanln(&sql)

		// 检查是否退出
		if sql == "exit" {
			break
		}

		// 处理SQL语句
		s.handleSQL(sql)
	}

	return nil
}

// 处理SQL语句
func (s *MySQLMCPServer) handleSQL(sql string) {
	if len(sql) == 0 {
		return
	}

	// 检查是否是查询语句
	if s.isQuerySQL(sql) {
		result, err := s.dbManager.Query("default", sql)
		if err != nil {
			fmt.Printf("查询失败: %v\n", err)
			return
		}

		// 打印列名
		for i, col := range result.Columns {
			if i > 0 {
				fmt.Print("\t")
			}
			fmt.Print(col)
		}
		fmt.Println()

		// 打印分隔线
		for i := 0; i < len(result.Columns); i++ {
			if i > 0 {
				fmt.Print("\t")
			}
			fmt.Print("--------")
		}
		fmt.Println()

		// 打印数据行
		for _, row := range result.Rows {
			for i, val := range row {
				if i > 0 {
					fmt.Print("\t")
				}
				fmt.Print(val)
			}
			fmt.Println()
		}
	} else {
		// 执行非查询语句
		result, err := s.dbManager.Execute("default", sql)
		if err != nil {
			fmt.Printf("执行失败: %v\n", err)
			return
		}

		fmt.Printf("影响行数: %d, 最后插入ID: %d\n", result.RowsAffected, result.LastInsertID)
	}
}

// 判断是否是查询SQL
func (s *MySQLMCPServer) isQuerySQL(sql string) bool {
	// 简单判断，以 SELECT 开头的认为是查询
	sql = s.trimSQL(sql)
	if len(sql) >= 6 && (sql[:6] == "SELECT" || sql[:6] == "select") {
		return true
	}
	return false
}

// 清理SQL首尾空白
func (s *MySQLMCPServer) trimSQL(sql string) string {
	// 移除首尾空白
	start := 0
	end := len(sql)

	// 移除开头空白
	for start < end && (sql[start] == ' ' || sql[start] == '\t' || sql[start] == '\n' || sql[start] == '\r') {
		start++
	}

	// 移除结尾空白
	for end > start && (sql[end-1] == ' ' || sql[end-1] == '\t' || sql[end-1] == '\n' || sql[end-1] == '\r') {
		end--
	}

	if start < end {
		return sql[start:end]
	}
	return ""
}
