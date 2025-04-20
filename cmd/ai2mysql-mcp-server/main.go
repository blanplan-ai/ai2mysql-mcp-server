package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blanplan-ai/ai2mysql-mcp-server/pkg/config"
	"github.com/blanplan-ai/ai2mysql-mcp-server/pkg/db"
)

var (
	configPath string
	debugMode  bool
)

func init() {
	flag.StringVar(&configPath, "config", "config.json", "配置文件路径")
	flag.BoolVar(&debugMode, "debug", true, "启用调试模式，将日志输出到/tmp/ai2mysql.log")
	
	// 直接解析环境变量中的JSON配置
	parseEnvConfigs()
}

// 解析环境变量中的JSON配置
func parseEnvConfigs() {
	// 检查是否有完整的env配置
	envStr := os.Getenv("env")
	if envStr != "" {
		println("找到env环境变量:", envStr)
		
		// 尝试解析为JSON对象
		var envConfig map[string]interface{}
		if err := json.Unmarshal([]byte(envStr), &envConfig); err == nil {
			// 检查args字段
			if args, ok := envConfig["args"].([]interface{}); ok {
				for _, arg := range args {
					if argStr, ok := arg.(string); ok {
						if argStr == "debug=true" {
							debugMode = true
							println("从env.args JSON对象设置debug=true")
						}
					}
				}
			}
			
			// 检查databases字段
			println("env中是否包含databases字段:", envConfig["databases"] != nil)
			
			// 检查permission字段
			println("env中是否包含permission字段:", envConfig["permission"] != nil)
		} else {
			println("解析env JSON失败:", err.Error())
		}
	}
}

func main() {
	// 尝试检测是否在MCP环境中
	isMCPEnv := os.Getenv("env") != "" || os.Getenv("env.args") != "" || os.Getenv("env.databases") != "" || 
	            os.Getenv("MYSQL_HOST") != "" || os.Getenv("MYSQL_USER") != ""
	
	// 输出重要环境信息
	println("MCP环境检测:", isMCPEnv)
	println("当前debug模式:", debugMode)
	println("命令行参数:", os.Args)
	
	// 处理命令行参数，包括自定义格式的参数
	processArgs()
	
	// 如果是MCP环境，且参数包含debug=true，强制启用debug模式
	if isMCPEnv {
		for _, arg := range os.Args {
			if arg == "debug=true" {
				debugMode = true
				println("从MCP命令行参数设置debug=true")
			}
		}
	}
	
	// 设置日志输出 - 在任何配置加载之前先设置
	if debugMode {
		// 立即设置日志输出到文件
		logFile, err := os.OpenFile("/tmp/ai2mysql.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			log.SetOutput(logFile)
			log.Printf("调试模式已启用，日志输出到 /tmp/ai2mysql.log")
		} else {
			log.Printf("无法打开日志文件: %v", err)
		}
	}

	// 打印所有命令行参数和环境变量，帮助诊断
	log.Printf("命令行参数: %v", os.Args)
	log.Printf("debug模式: %v", debugMode)
	log.Printf("配置路径: %s", configPath)
	
	// 检查所有可能的环境变量
	log.Printf("环境变量 MYSQL_HOST: %s", os.Getenv("MYSQL_HOST"))
	log.Printf("环境变量 MYSQL_PORT: %s", os.Getenv("MYSQL_PORT"))
	log.Printf("环境变量 MYSQL_USER: %s", os.Getenv("MYSQL_USER"))
	log.Printf("环境变量 MYSQL_PASS: %s", os.Getenv("MYSQL_PASS"))
	log.Printf("环境变量 MYSQL_DB: %s", os.Getenv("MYSQL_DB"))
	log.Printf("环境变量 ALLOW_INSERT_OPERATION: %s", os.Getenv("ALLOW_INSERT_OPERATION"))
	log.Printf("环境变量 ALLOW_UPDATE_OPERATION: %s", os.Getenv("ALLOW_UPDATE_OPERATION"))
	log.Printf("环境变量 ALLOW_DELETE_OPERATION: %s", os.Getenv("ALLOW_DELETE_OPERATION"))
	
	// 也检查旧环境变量名
	log.Printf("环境变量 env.databases.default.host: %s", os.Getenv("env.databases.default.host"))
	log.Printf("环境变量 env.databases: %s", os.Getenv("env.databases"))
	log.Printf("环境变量 env.args: %s", os.Getenv("env.args"))
	log.Printf("环境变量 env: %s", os.Getenv("env"))

	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 如果没有从环境变量加载到配置，使用用户提供的值（保险措施）
	if cfg.Databases["default"].Host == "localhost" && 
	   cfg.Databases["default"].User == "root" && 
	   cfg.Databases["default"].Password == "" {
		
		// 检查是否有环境变量，但没有正确加载
		if os.Getenv("MYSQL_HOST") != "" || os.Getenv("MYSQL_USER") != "" {
			log.Printf("检测到环境变量未正确加载，将手动设置配置")
			
			// 手动设置数据库配置
			dbConfig := cfg.Databases["default"]
			
			// 使用环境变量值，或保持默认值
			if host := os.Getenv("MYSQL_HOST"); host != "" {
				dbConfig.Host = host
			}
			
			if portStr := os.Getenv("MYSQL_PORT"); portStr != "" {
				if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
					dbConfig.Port = port
				}
			}
			
			if user := os.Getenv("MYSQL_USER"); user != "" {
				dbConfig.User = user
			}
			
			if pass := os.Getenv("MYSQL_PASS"); pass != "" {
				dbConfig.Password = pass
			}
			
			if db := os.Getenv("MYSQL_DB"); db != "" {
				dbConfig.DBName = db
			}
			
			cfg.Databases["default"] = dbConfig
			
			// 更新权限设置
			cfg.Permission.AllowQuery = true
			cfg.Permission.AllowInsert = parseBoolEnv(os.Getenv("ALLOW_INSERT_OPERATION"), false)
			cfg.Permission.AllowUpdate = parseBoolEnv(os.Getenv("ALLOW_UPDATE_OPERATION"), false)
			cfg.Permission.AllowDelete = parseBoolEnv(os.Getenv("ALLOW_DELETE_OPERATION"), false)
			
			log.Printf("已手动从环境变量设置配置")
		}
	}

	// 打印配置内容
	dbConfig, ok := cfg.Databases["default"]
	if ok {
		log.Printf("数据库配置: host=%s, port=%d, user=%s, db=%s",
			dbConfig.Host, dbConfig.Port, dbConfig.User, dbConfig.DBName)
		log.Printf("使用密码: %s", dbConfig.Password)
	} else {
		log.Printf("未找到默认数据库配置")
	}
	log.Printf("权限配置: query=%v, insert=%v, update=%v, delete=%v",
		cfg.Permission.AllowQuery, cfg.Permission.AllowInsert, 
		cfg.Permission.AllowUpdate, cfg.Permission.AllowDelete)

	// 打印配置来源信息
	configSourceMsg := "使用手动设置的配置"
	log.Printf("%s", configSourceMsg)

	// 创建数据库管理器
	dbManager, err := db.NewDBManager(cfg)
	if err != nil {
		log.Fatalf("初始化数据库管理器失败: %v", err)
	}
	defer dbManager.Close()

	// 如果配置文件不存在并且没有通过环境变量配置，创建默认配置文件
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
	server := NewMySQLMCPServer(dbManager, cfg)
	
	log.Printf("MySQL MCP 服务器已启动，等待连接...")
	if debugMode {
		log.Printf("调试模式已启用")
	}

	if err := server.Run(); err != nil {
		log.Fatalf("服务器运行失败: %v", err)
	}
}

// 处理命令行参数，包括支持key=value格式
func processArgs() {
	// 打印原始参数，用于诊断
	println("原始参数列表:")
	for i, arg := range os.Args {
		println(i, ":", arg)
	}

	// 首先处理标准flag参数
	flag.Parse()

	// 检查是否有未处理的参数
	for _, arg := range flag.Args() {
		println("处理未解析的参数:", arg)
		
		// 处理格式为"debug=true"的参数
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			key := strings.ToLower(parts[0])
			value := parts[1]

			// 处理debug=true这样的格式
			switch key {
			case "debug":
				println("找到debug参数:", value)
				if value == "true" || value == "1" || value == "yes" || value == "y" {
					debugMode = true
					println("设置debugMode = true")
				}
			case "config":
				configPath = value
				println("设置configPath =", value)
			}
		} else {
			// 处理可能的debug参数
			arg = strings.ToLower(arg)
			if arg == "debug" || arg == "debug=true" {
				debugMode = true
				println("通过独立参数设置debugMode = true")
			}
		}
	}

	// 处理os.Args中的参数，这些可能直接通过MCP服务器args传递
	for _, arg := range os.Args {
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			key := strings.ToLower(parts[0])
			value := parts[1]

			switch key {
			case "debug":
				println("从os.Args找到debug参数:", value)
				if value == "true" || value == "1" || value == "yes" || value == "y" {
					debugMode = true
					println("从os.Args设置debugMode = true")
				}
			}
		} else if strings.ToLower(arg) == "debug" {
			debugMode = true
			println("从os.Args设置debugMode = true（无值）")
		}
	}

	// 检查环境变量中的debug标志
	debugEnv := os.Getenv("DEBUG")
	if debugEnv == "" {
		// 尝试不同格式的环境变量名
		debugEnv = os.Getenv("debug")
	}
	if debugEnv != "" {
		println("从环境变量找到DEBUG:", debugEnv)
		if debugEnv == "true" || debugEnv == "1" || debugEnv == "yes" || debugEnv == "y" {
			debugMode = true
			println("从环境变量设置debugMode = true")
		}
	}
	
	// 检查env.args中可能存在的debug标志
	argsEnv := os.Getenv("env.args")
	if argsEnv != "" {
		println("找到env.args:", argsEnv)
		if strings.Contains(argsEnv, "debug=true") || strings.Contains(argsEnv, "debug") {
			debugMode = true
			println("从env.args设置debugMode = true")
		}
		
		// 尝试解析JSON格式的args
		var args []string
		if err := json.Unmarshal([]byte(argsEnv), &args); err == nil {
			println("成功解析env.args为JSON数组")
			for _, arg := range args {
				if arg == "debug" || arg == "debug=true" {
					debugMode = true
					println("从env.args JSON设置debugMode = true")
				}
			}
		}
	}
	
	// 最后检查一下环境变量里的args数组
	envJson := os.Getenv("env")
	if envJson != "" {
		var envConfig struct {
			Args []string `json:"args"`
		}
		if err := json.Unmarshal([]byte(envJson), &envConfig); err == nil && len(envConfig.Args) > 0 {
			println("从env JSON解析到args数组")
			for _, arg := range envConfig.Args {
				if arg == "debug" || arg == "debug=true" || strings.HasPrefix(arg, "debug=") {
					debugMode = true
					println("从env.args数组设置debugMode = true")
				}
			}
		}
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

// 解析环境变量中的布尔值
func parseBoolEnv(env string, defaultValue bool) bool {
	if env == "" {
		return defaultValue
	}
	if strings.ToLower(env) == "true" || strings.ToLower(env) == "1" || strings.ToLower(env) == "yes" || strings.ToLower(env) == "y" {
		return true
	}
	return false
}
