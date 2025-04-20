package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

// DBConfig 包含单个数据库的配置信息
type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

// Permission 权限配置
type Permission struct {
	AllowQuery  bool `json:"allow_query"`
	AllowInsert bool `json:"allow_insert"`
	AllowUpdate bool `json:"allow_update"`
	AllowDelete bool `json:"allow_delete"`
}

// Config 应用配置结构
type Config struct {
	Databases  map[string]DBConfig `json:"databases"`
	Permission Permission          `json:"permission"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Databases: map[string]DBConfig{
			"default": {
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Password: "",
				DBName:   "test",
			},
		},
		Permission: Permission{
			AllowQuery:  true,
			AllowInsert: false,
			AllowUpdate: false,
			AllowDelete: false,
		},
	}
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	// 首先尝试从环境变量加载配置
	if envConfig := LoadConfigFromEnv(); envConfig != nil {
		return envConfig, nil
	}
	
	// 尝试从MCP服务器参数的env字段加载配置
	if mcpEnvConfig := LoadConfigFromMCPEnv(); mcpEnvConfig != nil {
		return mcpEnvConfig, nil
	}

	// 如果环境变量配置不存在，尝试从文件加载
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 配置文件不存在，返回默认配置
		return DefaultConfig(), nil
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 如果没有配置数据库，使用默认配置
	if len(config.Databases) == 0 {
		defaultConfig := DefaultConfig()
		config.Databases = defaultConfig.Databases
	}

	return &config, nil
}

// LoadConfigFromEnv 从环境变量加载配置
func LoadConfigFromEnv() *Config {
	// 首先尝试获取嵌套的JSON格式环境变量配置
	if envConfig := loadConfigFromJsonEnv(); envConfig != nil {
		return envConfig
	}

	// 读取环境变量（支持多种可能的环境变量名称）
	host := os.Getenv("MYSQL_HOST")
	
	// 尝试解析端口
	var port int = 3306 // 默认端口
	portStr := os.Getenv("MYSQL_PORT")
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			port = p
		}
	}
	
	// 获取用户名
	user := os.Getenv("MYSQL_USER")
	
	// 尝试获取密码（支持多种可能的环境变量名）
	pass := os.Getenv("MYSQL_PASS")
	if pass == "" {
		pass = os.Getenv("MYSQL_PASSWORD") // 尝试备选名称
	}
	
	// 尝试获取数据库名（支持多种可能的环境变量名）
	dbName := os.Getenv("MYSQL_DB")
	if dbName == "" {
		dbName = os.Getenv("DEFAULT_DATABASE") // 尝试备选名称
	}
	
	// 获取权限配置（支持多种可能的环境变量名）
	allowInsertStr := os.Getenv("ALLOW_INSERT_OPERATION")
	if allowInsertStr == "" {
		allowInsertStr = os.Getenv("ALLOW_INSERT")
	}
	
	allowUpdateStr := os.Getenv("ALLOW_UPDATE_OPERATION")
	if allowUpdateStr == "" {
		allowUpdateStr = os.Getenv("ALLOW_UPDATE")
	}
	
	allowDeleteStr := os.Getenv("ALLOW_DELETE_OPERATION")
	if allowDeleteStr == "" {
		allowDeleteStr = os.Getenv("ALLOW_DELETE")
	}

	// 检查必要的环境变量是否存在
	if host == "" && user == "" {
		return nil // 没有环境变量配置
	}

	// 使用默认配置作为基础
	config := DefaultConfig()

	// 创建默认数据库配置
	dbConfig := config.Databases["default"]

	// 设置数据库连接信息
	if host != "" {
		dbConfig.Host = host
	}

	// 直接设置端口（已经处理过错误情况）
	if port > 0 {
		dbConfig.Port = port
	}

	if user != "" {
		dbConfig.User = user
	}

	if pass != "" {
		dbConfig.Password = pass
	}

	if dbName != "" {
		dbConfig.DBName = dbName
	}

	// 更新数据库配置
	config.Databases["default"] = dbConfig

	// 设置权限
	// 查询权限始终允许
	config.Permission.AllowQuery = true

	// 解析其他权限配置
	config.Permission.AllowInsert = parseBoolEnv(allowInsertStr, false)
	config.Permission.AllowUpdate = parseBoolEnv(allowUpdateStr, false)
	config.Permission.AllowDelete = parseBoolEnv(allowDeleteStr, false)

	return config
}

// loadConfigFromJsonEnv 从JSON结构的环境变量中获取配置
func loadConfigFromJsonEnv() *Config {
	// 检查是否有数据库配置环境变量
	databasesEnv := os.Getenv("databases")
	permissionEnv := os.Getenv("permission")
	
	// 嵌套环境变量解析 - 处理MCP启动配置中的env结构
	// 在这种情况下，用户使用的是类似于以下格式：
	// env: {
	//   databases: { default: {...} },
	//   permission: {...}
	// }
	if databasesEnv == "" && permissionEnv == "" {
		// 检查是否有嵌套在env下的配置
		dbJsonStr := os.Getenv("env.databases")
		permJsonStr := os.Getenv("env.permission")
		
		if dbJsonStr != "" || permJsonStr != "" {
			config := DefaultConfig()
			
			// 处理数据库配置
			if dbJsonStr != "" {
				var databases map[string]DBConfig
				if err := json.Unmarshal([]byte(dbJsonStr), &databases); err == nil && len(databases) > 0 {
					config.Databases = databases
				}
			}
			
			// 处理权限配置
			if permJsonStr != "" {
				var permission Permission
				if err := json.Unmarshal([]byte(permJsonStr), &permission); err == nil {
					config.Permission = permission
				}
			}
			
			return config
		}
		
		// 尝试解析完整的env字段
		envJsonStr := os.Getenv("env")
		if envJsonStr != "" {
			var configData struct {
				Databases  map[string]DBConfig `json:"databases"`
				Permission Permission          `json:"permission"`
			}
			
			if err := json.Unmarshal([]byte(envJsonStr), &configData); err == nil {
				config := DefaultConfig()
				
				if len(configData.Databases) > 0 {
					config.Databases = configData.Databases
				}
				
				// 只覆盖非空的权限配置
				config.Permission = configData.Permission
				
				return config
			}
		}
		
		return nil
	}

	config := DefaultConfig()

	// 处理数据库配置
	if databasesEnv != "" {
		var databases map[string]DBConfig
		if err := json.Unmarshal([]byte(databasesEnv), &databases); err == nil && len(databases) > 0 {
			config.Databases = databases
		}
	}

	// 处理权限配置
	if permissionEnv != "" {
		var permission Permission
		if err := json.Unmarshal([]byte(permissionEnv), &permission); err == nil {
			config.Permission = permission
		}
	}

	return config
}

// parseBoolEnv 解析布尔环境变量值
func parseBoolEnv(value string, defaultVal bool) bool {
	if value == "" {
		return defaultVal
	}

	value = strings.ToLower(value)
	return value == "true" || value == "1" || value == "yes" || value == "y"
}

// SaveConfig 保存配置到文件
func SaveConfig(config *Config, path string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0644)
}

// LoadConfigFromMCPEnv 从MCP服务器配置的env字段加载配置
func LoadConfigFromMCPEnv() *Config {
	// 尝试解析MCP配置中的数据库和权限
	dbsJsonStr := os.Getenv("env.databases.default.host")
	if dbsJsonStr != "" {
		// 发现嵌套的MCP环境变量格式，尝试构建配置
		config := DefaultConfig()
		dbConfig := DBConfig{
			Host:     os.Getenv("env.databases.default.host"),
			User:     os.Getenv("env.databases.default.user"),
			Password: os.Getenv("env.databases.default.password"),
			DBName:   os.Getenv("env.databases.default.dbname"),
		}
		
		// 尝试解析端口
		portStr := os.Getenv("env.databases.default.port")
		if portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
				dbConfig.Port = port
			}
		}
		
		// 更新数据库配置
		config.Databases["default"] = dbConfig
		
		// 处理权限配置
		config.Permission.AllowQuery = parseBoolEnv(os.Getenv("env.permission.allow_query"), true)
		config.Permission.AllowInsert = parseBoolEnv(os.Getenv("env.permission.allow_insert"), false)
		config.Permission.AllowUpdate = parseBoolEnv(os.Getenv("env.permission.allow_update"), false)
		config.Permission.AllowDelete = parseBoolEnv(os.Getenv("env.permission.allow_delete"), false)
		
		return config
	}
	
	return nil
}
