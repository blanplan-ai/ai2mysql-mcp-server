# MySQL MCP Server

A MySQL database connection server built on [go-mcp](https://github.com/ThinkInAIXYZ/go-mcp), supporting communication with clients via stdio, allowing execution of SQL queries and data manipulation operations.

## Features

- Communicates with clients through the MCP protocol
- Supports connection to MySQL databases
- Supports query operations (SELECT, SHOW, DESCRIBE)
- Supports data manipulation operations (INSERT/UPDATE/DELETE)
- Permission control (configurable for insert, update, delete operations)
- Flexible configuration via environment variables
- Debug mode with detailed logging to file

## Installation

### Method 1: Direct Installation (Recommended)

Install directly from GitHub using Go's install command:

```bash
go install github.com/blanplan-ai/ai2mysql-mcp-server/cmd/ai2mysql-mcp-server@latest
```

### Method 2: Manual Build

```bash
# Clone the repository
git clone https://github.com/blanplan-ai/ai2mysql-mcp-server.git
cd ai2mysql-mcp-server

# Build the server
go build -o ai2mysql-mcp-server ./cmd/ai2mysql-mcp-server
```

## Configuration

The server is configured using environment variables.

### Environment Variables

The following environment variables can be set to configure the server:

| Environment Variable | Description | Default Value |
|---------|------|--------|
| `MYSQL_HOST` | MySQL host address | 127.0.0.1 |
| `MYSQL_PORT` | MySQL port number | 3306 |
| `MYSQL_USER` | MySQL username | root |
| `MYSQL_PASS` | MySQL password | password |
| `DEFAULT_DATABASE` | Default database name | test |
| `ALLOW_INSERT` | Whether to allow INSERT operations | false |
| `ALLOW_UPDATE` | Whether to allow UPDATE operations | false |
| `ALLOW_DELETE` | Whether to allow DELETE operations | false |
| `IS_DEV` | Enable development mode with detailed logging | false |
| `LOG_PATH` | Path to the log file (only effective in dev mode) | /tmp/ai2mysql.log |

### JSON Configuration Example

Below is an example JSON configuration you can use to set environment variables:

```json
{
  "MYSQL_HOST": "127.0.0.1",
  "MYSQL_PORT": "3306",
  "MYSQL_USER": "root",
  "MYSQL_PASS": "password",
  "DEFAULT_DATABASE": "test",
  "ALLOW_INSERT": "false",
  "ALLOW_UPDATE": "false",
  "ALLOW_DELETE": "false",
  "IS_DEV": "false",
  "LOG_PATH": "/tmp/ai2mysql.log"
}
```

Configuration notes:

- All boolean type configuration items use the string "true" or "false"
- In production environments, it is recommended to disable IS_DEV to prevent sensitive information leakage
- It is recommended to restrict INSERT/UPDATE/DELETE permissions based on actual needs
- Sensitive information such as passwords should be configured using environment variables

## Usage

### Starting the Server

Run the server after installation:

```bash
# Run with default settings
ai2mysql-mcp-server

# Run with environment variables
MYSQL_HOST=127.0.0.1 MYSQL_USER=root MYSQL_PASS=password ai2mysql-mcp-server

# Enable development mode
IS_DEV=true ai2mysql-mcp-server
```

When development mode is enabled, detailed logs will be output to the configured log file (default: `/tmp/ai2mysql.log`), including complete request, response, and error information.

### Using with Cursor

This server is designed to work with clients that support the MCP protocol, such as Cursor.

In Cursor, you can enable the MySQL MCP server with the following configuration:

```json
{
  "mcpServers": {
    "mysql-mcp-server": {
      "command": "ai2mysql-mcp-server",
      "args": [],
      "env": {
        "MYSQL_HOST": "127.0.0.1",
        "MYSQL_PORT": "3306",
        "MYSQL_USER": "root",
        "MYSQL_PASS": "password",
        "DEFAULT_DATABASE": "test",   // Default database, optional
        "ALLOW_INSERT": "false",      // Allow insert operations, optional
        "ALLOW_UPDATE": "false",      // Allow update operations, optional
        "ALLOW_DELETE": "false",      // Allow delete operations, optional
        "IS_DEV": "false"             // Enable development mode, optional
      }
    }
  }
}
```

After adding the configuration, you can use the following tools:

1. `mysql_query`: Execute SQL query operations
   - Parameter: `sql` - SQL query statement (SELECT, SHOW, DESCRIBE)

2. `mysql_execute`: Execute SQL data manipulation operations
   - Parameter: `sql` - SQL data manipulation statement (INSERT/UPDATE/DELETE)

## Development

### Project Structure

```
ai2mysql-mcp-server/
└── cmd/
    └── ai2mysql-mcp-server/  # Server entry
        └── main.go          # Main program
```

### Dependencies

- [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql): MySQL driver
- [ThinkInAIXYZ/go-mcp](https://github.com/ThinkInAIXYZ/go-mcp): MCP protocol library

## Debugging and Troubleshooting

### Debug Mode

Set the environment variable `IS_DEV=true` to enable debug mode:

```bash
IS_DEV=true ai2mysql-mcp-server
```

In debug mode, the server will output detailed logs to the specified log file, including:

- All received requests and sent responses
- Detailed SQL query and execution information
- Error and exception conditions
- Performance-related information (such as query execution time)

### Common Issues

If you encounter problems in Cursor:

1. **Tool not displayed**: Check if the command path in the Cursor configuration file is correct, ensure the executable exists and has execution permissions
2. **Connection failed**: Check if the database connection information is correct, use debug mode to view detailed error messages
3. **Permission issues**: By default only query operations are allowed, check if you need to enable insert/update/delete permissions

## License

MIT License