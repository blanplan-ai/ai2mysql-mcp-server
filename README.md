# MySQL MCP Server

基于 [Model Context Protocol (MCP)](https://github.com/ThinkInAIXYZ/go-mcp) 的 MySQL 数据库连接服务器，支持通过 stdio 方式与客户端通信，允许执行 SQL 查询和数据操作。

## 功能特性

- 通过 MCP 协议与客户端通信
- 支持连接多个 MySQL 数据库
- 支持查询操作（SELECT）
- 支持数据操作（INSERT/UPDATE/DELETE）
- 权限控制（可分别配置查询、插入、更新、删除权限）
- 支持通过环境变量或JSON配置文件进行灵活配置

## 安装

### 方法一：直接安装（推荐）

使用 Go 的安装命令直接从 GitHub 安装：

```bash
go install github.com/blanplan-ai/ai2mysql-mcp-server/cmd/ai2mysql-mcp-server@latest
```

安装完成后，可以直接在命令行中运行：

```bash
ai2mysql-mcp-server
```

### 方法二：手动构建

```bash
# 克隆仓库
git clone https://github.com/blanplan-ai/ai2mysql-mcp-server.git
cd ai2mysql-mcp-server

# 构建服务器
go build -o ai2mysql-mcp-server ./cmd/ai2mysql-mcp-server
```

## 配置

服务器支持两种配置方式：环境变量和JSON配置文件。当环境变量存在时，会优先使用环境变量的配置。

### 环境变量配置

可以设置以下环境变量来配置服务器：

| 环境变量 | 说明 | 默认值 |
|---------|------|--------|
| `MYSQL_HOST` | MySQL 主机地址 | localhost |
| `MYSQL_PORT` | MySQL 端口号 | 3306 |
| `MYSQL_USER` | MySQL 用户名 | root |
| `MYSQL_PASS` | MySQL 密码 | (空) |
| `DEFAULT_DATABASE` | 默认数据库名 | test |
| `ALLOW_INSERT` | 是否允许插入操作 | false |
| `ALLOW_UPDATE` | 是否允许更新操作 | false |
| `ALLOW_DELETE` | 是否允许删除操作 | false |

### JSON配置文件

如果未设置环境变量，服务器将使用JSON配置文件。默认配置文件名为 `config.json`，可以通过 `-config` 参数指定其他配置文件。

配置文件示例：

```json
{
  "databases": {
    "default": {
      "host": "localhost",
      "port": 3306,
      "user": "root",
      "password": "",
      "dbname": "test"
    },
    "other_db": {
      "host": "localhost",
      "port": 3306,
      "user": "root",
      "password": "",
      "dbname": "other_database"
    }
  },
  "permission": {
    "allow_query": true,
    "allow_insert": false,
    "allow_update": false,
    "allow_delete": false
  }
}
```

配置说明：

- `databases`: 配置多个数据库连接
  - `default`: 默认数据库（必需）
  - 可以添加其他数据库配置
- `permission`: 权限配置
  - `allow_query`: 是否允许查询操作（默认：true）
  - `allow_insert`: 是否允许插入操作（默认：false）
  - `allow_update`: 是否允许更新操作（默认：false）
  - `allow_delete`: 是否允许删除操作（默认：false）

## 使用方法

### 启动服务器

安装后直接运行：

```bash
# 使用默认配置文件
ai2mysql-mcp-server

# 指定配置文件
ai2mysql-mcp-server -config=/path/to/config.json

# 使用环境变量配置
MYSQL_HOST=127.0.0.1 MYSQL_USER=root MYSQL_PASS=password ai2mysql-mcp-server
```

### 在 Cursor 中使用

本服务器设计为与支持 MCP 协议的客户端（如 Cursor）配合使用。

在 Cursor 中，可以通过以下配置启用 MySQL MCP 服务器：

```json
{
  "mcpServers": {
    "ai2mysql-mcp-server": {
      "command": "ai2mysql-mcp-server",
      "args": [],
      "env": {
        "MYSQL_HOST": "127.0.0.1",
        "MYSQL_PORT": "3306",
        "MYSQL_USER": "root",
        "MYSQL_PASS": "password",
        "DEFAULT_DATABASE": "test",  // 默认数据库，可选
        "ALLOW_INSERT": "false",     // 是否允许插入，可选
        "ALLOW_UPDATE": "false",     // 是否允许更新，可选
        "ALLOW_DELETE": "false"      // 是否允许删除，可选
      }
    }
  }
}
```

添加配置后，你可以通过工具调用使用以下功能：

1. `mcp_mysql_query`: 执行 SQL 查询操作
   - 参数: `sql` - SQL 查询语句

2. `mcp_mysql_execute`: 执行 SQL 数据操作
   - 参数: `sql` - SQL 数据操作语句（INSERT/UPDATE/DELETE）

## 开发

### 项目结构

```
ai2mysql-mcp-server/
├── cmd/
│   └── ai2mysql-mcp-server/  # 服务器入口
│       ├── main.go          # 主程序
│       └── mcp_server.go    # MCP 服务器实现
├── pkg/
│   ├── config/              # 配置管理
│   │   └── config.go
│   └── db/                  # 数据库操作
│       └── db.go
├── go.mod                   # Go 模块定义
├── go.sum                   # 依赖校验和
├── config.json.example      # 配置文件示例
└── README.md                # 说明文档
```

### 依赖

- [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql): MySQL 驱动
- [ThinkInAIXYZ/go-mcp](https://github.com/ThinkInAIXYZ/go-mcp): MCP 协议库

## 许可证

MIT License