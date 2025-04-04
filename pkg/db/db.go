package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/blanplan-ai/ai2mysql-mcp-server/pkg/config"
	_ "github.com/go-sql-driver/mysql"
)

// DBManager 数据库连接管理器
type DBManager struct {
	connections map[string]*sql.DB
	config      *config.Config
}

// QueryResult 查询结果结构
type QueryResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Error   string          `json:"error,omitempty"`
}

// ExecuteResult 执行结果结构
type ExecuteResult struct {
	RowsAffected int64  `json:"rows_affected"`
	LastInsertID int64  `json:"last_insert_id"`
	Error        string `json:"error,omitempty"`
}

// NewDBManager 创建数据库管理器实例
func NewDBManager(cfg *config.Config) (*DBManager, error) {
	manager := &DBManager{
		connections: make(map[string]*sql.DB),
		config:      cfg,
	}

	// 连接所有配置的数据库
	for name, dbConfig := range cfg.Databases {
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			dbConfig.User,
			dbConfig.Password,
			dbConfig.Host,
			dbConfig.Port,
			dbConfig.DBName,
		)

		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("连接数据库 %s 失败: %v", name, err)
		}

		// 设置连接池参数
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(time.Minute * 3)

		// 测试连接
		if err := db.Ping(); err != nil {
			return nil, fmt.Errorf("ping 数据库 %s 失败: %v", name, err)
		}

		manager.connections[name] = db
	}

	return manager, nil
}

// Close 关闭所有数据库连接
func (m *DBManager) Close() {
	for _, db := range m.connections {
		db.Close()
	}
}

// GetDB 获取指定名称的数据库连接
func (m *DBManager) GetDB(name string) (*sql.DB, error) {
	db, ok := m.connections[name]
	if !ok {
		return nil, fmt.Errorf("数据库 %s 未配置", name)
	}
	return db, nil
}

// Query 执行查询操作
func (m *DBManager) Query(dbName, query string, args ...interface{}) (*QueryResult, error) {
	if !m.config.Permission.AllowQuery {
		return nil, fmt.Errorf("查询操作未被允许")
	}

	db, err := m.GetDB(dbName)
	if err != nil {
		return &QueryResult{Error: err.Error()}, err
	}

	// 执行查询
	rows, err := db.Query(query, args...)
	if err != nil {
		return &QueryResult{Error: err.Error()}, err
	}
	defer rows.Close()

	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return &QueryResult{Error: err.Error()}, err
	}

	// 准备结果
	result := &QueryResult{
		Columns: columns,
		Rows:    make([][]interface{}, 0),
	}

	// 遍历结果集
	for rows.Next() {
		// 创建一个列值的slice
		values := make([]interface{}, len(columns))
		// 创建一个接收指针的slice
		valuePtrs := make([]interface{}, len(columns))

		// 初始化接收指针
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		// 扫描当前行
		if err := rows.Scan(valuePtrs...); err != nil {
			return &QueryResult{Error: err.Error()}, err
		}

		// 处理结果，转换为合适的类型
		row := make([]interface{}, len(columns))
		for i, v := range values {
			if v == nil {
				row[i] = nil
				continue
			}

			// 根据数据类型转换
			switch v.(type) {
			case []byte:
				row[i] = string(v.([]byte))
			default:
				row[i] = v
			}
		}

		result.Rows = append(result.Rows, row)
	}

	// 检查遍历过程中是否有错误
	if err := rows.Err(); err != nil {
		return &QueryResult{Error: err.Error()}, err
	}

	return result, nil
}

// Execute 执行插入/更新/删除操作
func (m *DBManager) Execute(dbName, query string, args ...interface{}) (*ExecuteResult, error) {
	// 根据SQL语句类型检查权限
	lowerQuery := strings.TrimSpace(strings.ToLower(query))

	if strings.HasPrefix(lowerQuery, "insert") {
		if !m.config.Permission.AllowInsert {
			return nil, fmt.Errorf("插入操作未被允许")
		}
	} else if strings.HasPrefix(lowerQuery, "update") {
		if !m.config.Permission.AllowUpdate {
			return nil, fmt.Errorf("更新操作未被允许")
		}
	} else if strings.HasPrefix(lowerQuery, "delete") {
		if !m.config.Permission.AllowDelete {
			return nil, fmt.Errorf("删除操作未被允许")
		}
	}

	db, err := m.GetDB(dbName)
	if err != nil {
		return &ExecuteResult{Error: err.Error()}, err
	}

	// 执行操作
	result, err := db.Exec(query, args...)
	if err != nil {
		return &ExecuteResult{Error: err.Error()}, err
	}

	// 获取影响的行数
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &ExecuteResult{Error: err.Error()}, err
	}

	// 获取最后插入的ID
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return &ExecuteResult{Error: err.Error()}, err
	}

	return &ExecuteResult{
		RowsAffected: rowsAffected,
		LastInsertID: lastInsertID,
	}, nil
}
