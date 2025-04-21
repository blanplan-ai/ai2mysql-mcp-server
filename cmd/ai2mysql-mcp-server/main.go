// MySQL MCP Server - MySQL database access tools
//
// Usage:
//
//	mysql_mcp_server
//
// Supported tools:
//   - mysql_query: Execute MySQL queries (read-only, SELECT statements)
//   - mysql_execute: Execute MySQL update operations (non-query statements)
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
	_ "github.com/go-sql-driver/mysql"
)

// Database connection
var db *sql.DB

// Permission control flags
var allowInsert, allowUpdate, allowDelete bool

// Development mode flag, controls whether to print detailed logs
var isDev bool

// Log enabled flag
var logEnabled bool

func main() {
	flag.Parse()

	// Set development mode flag
	isDev = getEnvWithDefault("IS_DEV", "false") == "true"

	// Configure log output
	logEnabled = false
	if isDev {
		// Read log path configuration
		logPath := getEnvWithDefault("LOG_PATH", "/tmp/ai2mysql.log")

		// Try to open log file
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			// Successfully opened log file
			log.SetOutput(logFile)
			logEnabled = true
			defer logFile.Close()
			log.Printf("Logging enabled, output to: %s", logPath)
		} else {
			// If unable to open log file, disable logging
			log.SetOutput(os.Stderr)
			log.Printf("Warning: Unable to open log file %s: %v, logging will be disabled", logPath, err)
			// Redirect logs to null device
			log.SetOutput(&nullWriter{})
		}
	} else {
		// In non-development mode, disable logging
		log.SetOutput(&nullWriter{})
	}

	// Read environment variables
	mysqlHost := getEnvWithDefault("MYSQL_HOST", "127.0.0.1")
	mysqlPort := getEnvWithDefault("MYSQL_PORT", "3306")
	mysqlUser := getEnvWithDefault("MYSQL_USER", "root")
	mysqlPass := getEnvWithDefault("MYSQL_PASS", "password")
	defaultDB := getEnvWithDefault("DEFAULT_DATABASE", "test")

	// Set permission control flags
	allowInsert = getEnvWithDefault("ALLOW_INSERT", "false") == "true"
	allowUpdate = getEnvWithDefault("ALLOW_UPDATE", "false") == "true"
	allowDelete = getEnvWithDefault("ALLOW_DELETE", "false") == "true"

	// Log environment variable information, only in development mode
	if logEnabled {
		log.Printf("Environment variables configuration:")
		log.Printf("MYSQL_HOST: %s", mysqlHost)
		log.Printf("MYSQL_PORT: %s", mysqlPort)
		log.Printf("MYSQL_USER: %s", mysqlUser)
		log.Printf("MYSQL_PASS: %s", mysqlPass)
		log.Printf("DEFAULT_DATABASE: %s", defaultDB)
		log.Printf("ALLOW_INSERT: %v", allowInsert)
		log.Printf("ALLOW_UPDATE: %v", allowUpdate)
		log.Printf("ALLOW_DELETE: %v", allowDelete)
		log.Printf("IS_DEV: %v", isDev)
	}

	// Build DSN
	connectionDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		mysqlUser, mysqlPass, mysqlHost, mysqlPort, defaultDB)

	// Log DSN information, only in development mode
	if logEnabled {
		log.Printf("Using DSN: %s", connectionDSN)
		// Log full startup command
		log.Printf("Startup command: %v", os.Args)
	}

	// Initialize database
	if err := initDB(connectionDSN); err != nil {
		// Database connection failure is a critical error, should be logged to stderr even if logging is disabled
		log.SetOutput(os.Stderr)
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	// Create MCP server
	srv, err := server.NewServer(
		transport.NewStdioServerTransport(),
		server.WithServerInfo(protocol.Implementation{
			Name:    "mysql-mcp-server",
			Version: "1.0.0",
		}),
	)
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Fatalf("Server creation failed: %v", err)
	}

	// Register query tool
	srv.RegisterTool(&protocol.Tool{
		Name:        "mysql_query",
		Description: "Execute MySQL queries (read-only, SELECT statements)",
		InputSchema: protocol.InputSchema{
			Type: protocol.Object,
			Properties: map[string]*protocol.Property{
				"sql": {
					Type:        protocol.String,
					Description: "SQL query statement to execute",
				},
			},
			Required: []string{"sql"},
		},
	}, handleQuery)

	// Register execute tool
	srv.RegisterTool(&protocol.Tool{
		Name:        "mysql_execute",
		Description: "Execute MySQL update operations (INSERT/UPDATE/DELETE and other non-query statements)",
		InputSchema: protocol.InputSchema{
			Type: protocol.Object,
			Properties: map[string]*protocol.Property{
				"sql": {
					Type:        protocol.String,
					Description: "SQL statement to execute",
				},
			},
			Required: []string{"sql"},
		},
	}, handleExecute)

	// Start server
	if logEnabled {
		log.Println("Starting MySQL MCP Server with stdio transport mode")
	}
	if err = srv.Run(); err != nil {
		log.SetOutput(os.Stderr)
		log.Fatalf("Service runtime error: %v", err)
	}
}

// Null log writer, used to disable logging
type nullWriter struct{}

func (nw *nullWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// Get environment variable value, return default value if not exist
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Initialize database connection
func initDB(connectionDSN string) error {
	var err error
	db, err = sql.Open("mysql", connectionDSN)
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(60) // 1 minute

	return db.Ping()
}

// Handle MySQL query requests
func handleQuery(request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	startTime := time.Now()
	sql, ok := request.Arguments["sql"].(string)
	if !ok {
		return nil, errors.New("sql must be a string")
	}

	if logEnabled {
		log.Printf("[QUERY REQUEST] SQL: %s", sql)
	}

	// Ensure it's a read-only query
	sqlUpper := strings.TrimSpace(strings.ToUpper(sql))
	if !strings.HasPrefix(sqlUpper, "SELECT") && !strings.HasPrefix(sqlUpper, "SHOW") && !strings.HasPrefix(sqlUpper, "DESCRIBE") {
		if logEnabled {
			log.Printf("[QUERY REJECTED] Invalid query type: %s", sqlUpper[:10])
		}
		return nil, errors.New("only SELECT, SHOW, or DESCRIBE queries are allowed")
	}

	// Execute query
	rows, err := db.Query(sql)
	if err != nil {
		if logEnabled {
			log.Printf("[QUERY ERROR] %v", err)
		}
		return nil, fmt.Errorf("query execution error: %v", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		if logEnabled {
			log.Printf("[QUERY ERROR] Failed to get column names: %v", err)
		}
		return nil, fmt.Errorf("failed to get column names: %v", err)
	}

	// Process results
	var results []map[string]interface{}
	values := make([]interface{}, len(columns))
	scanArgs := make([]interface{}, len(columns))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	rowCount := 0
	for rows.Next() {
		rowCount++
		if err = rows.Scan(scanArgs...); err != nil {
			if logEnabled {
				log.Printf("[QUERY ERROR] Failed to read row data: %v", err)
			}
			return nil, fmt.Errorf("failed to read row data: %v", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err = rows.Err(); err != nil {
		if logEnabled {
			log.Printf("[QUERY ERROR] Error iterating through results: %v", err)
		}
		return nil, fmt.Errorf("error iterating through results: %v", err)
	}

	// Convert to JSON
	jsonData, err := json.Marshal(results)
	if err != nil {
		if logEnabled {
			log.Printf("[QUERY ERROR] JSON serialization failed: %v", err)
		}
		return nil, fmt.Errorf("JSON serialization failed: %v", err)
	}

	executionTime := time.Since(startTime)
	if logEnabled {
		log.Printf("[QUERY COMPLETED] Time: %v, Rows: %d, SQL: %s", executionTime, rowCount, sql)
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			protocol.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

// Handle MySQL execute requests
func handleExecute(request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	startTime := time.Now()
	sql, ok := request.Arguments["sql"].(string)
	if !ok {
		return nil, errors.New("sql must be a string")
	}

	if logEnabled {
		log.Printf("[EXECUTE REQUEST] SQL: %s", sql)
	}

	// Permission check
	sqlUpper := strings.TrimSpace(strings.ToUpper(sql))

	// Check INSERT permission
	if strings.HasPrefix(sqlUpper, "INSERT") && !allowInsert {
		if logEnabled {
			log.Printf("[EXECUTE REJECTED] No INSERT permission")
		}
		return nil, errors.New("no INSERT permission")
	}

	// Check UPDATE permission
	if strings.HasPrefix(sqlUpper, "UPDATE") && !allowUpdate {
		if logEnabled {
			log.Printf("[EXECUTE REJECTED] No UPDATE permission")
		}
		return nil, errors.New("no UPDATE permission")
	}

	// Check DELETE permission
	if strings.HasPrefix(sqlUpper, "DELETE") && !allowDelete {
		if logEnabled {
			log.Printf("[EXECUTE REJECTED] No DELETE permission")
		}
		return nil, errors.New("no DELETE permission")
	}

	// Prohibit dangerous operations
	if strings.HasPrefix(sqlUpper, "DROP") || strings.HasPrefix(sqlUpper, "TRUNCATE") {
		if logEnabled {
			log.Printf("[EXECUTE REJECTED] Dangerous operation not allowed: %s", sqlUpper[:10])
		}
		return nil, errors.New("DROP or TRUNCATE operations are not allowed")
	}

	// Execute SQL
	result, err := db.Exec(sql)
	if err != nil {
		if logEnabled {
			log.Printf("[EXECUTE ERROR] %v", err)
		}
		return nil, fmt.Errorf("SQL execution error: %v", err)
	}

	// Get results
	lastInsertID, _ := result.LastInsertId()
	rowsAffected, _ := result.RowsAffected()

	response := map[string]interface{}{
		"lastInsertId": lastInsertID,
		"rowsAffected": rowsAffected,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		if logEnabled {
			log.Printf("[EXECUTE ERROR] JSON serialization failed: %v", err)
		}
		return nil, fmt.Errorf("JSON serialization failed: %v", err)
	}

	executionTime := time.Since(startTime)
	if logEnabled {
		log.Printf("[EXECUTE COMPLETED] Time: %v, Rows affected: %d, Last insert ID: %d, SQL: %s",
			executionTime, rowsAffected, lastInsertID, sql)
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			protocol.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}
