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
	// 读取环境变量
	host := os.Getenv("MYSQL_HOST")
	portStr := os.Getenv("MYSQL_PORT")
	user := os.Getenv("MYSQL_USER")
	pass := os.Getenv("MYSQL_PASS")
	dbName := os.Getenv("DEFAULT_DATABASE")
	allowInsertStr := os.Getenv("ALLOW_INSERT")
	allowUpdateStr := os.Getenv("ALLOW_UPDATE")
	allowDeleteStr := os.Getenv("ALLOW_DELETE")

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

	if portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
			dbConfig.Port = port
		}
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
