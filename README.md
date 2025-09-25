# DB MCP Server

A Model Context Protocol (MCP) server that provides database connectivity and query capabilities for AI assistants. Connect to multiple databases (PostgreSQL, MySQL) and execute queries through a simple, secure interface.

## Features

- 🔗 **Multi-Database Support**: Connect to PostgreSQL and MySQL databases
- 🔧 **Multiple Connection Management**: Define and switch between named database connections
- 🛡️ **Security**: Read-only mode support with query validation
- 📊 **Rich Query Results**: Properly formatted JSON results with type handling
- ⚡ **Session Management**: Efficient connection pooling and reuse

## Installation

```bash
git clone https://github.com/AbdelilahOu/DBMcp.git
cd DBMcp
go build -o db-mcp-server ./cmd
```

## Configuration

Create a configuration file at one of these locations:
- `~/.config/db-mcp/connections.json` (Linux/macOS)
- `%APPDATA%\db-mcp\connections.json` (Windows)
- `./connections.json` (Current directory)

### Configuration File Format

```json
{
  "connections": {
    "production_pg": {
      "name": "Production PostgreSQL",
      "type": "postgres",
      "url": "postgres://user:password@localhost:5432/mydb?sslmode=disable",
      "description": "Main production database"
    },
    "dev_mysql": {
      "name": "Development MySQL",
      "type": "mysql",
      "url": "mysql://user:password@localhost:3306/testdb",
      "description": "Development MySQL instance"
    },
    "analytics": {
      "name": "Analytics DB",
      "type": "postgres",
      "url": "postgres://readonly:pass@analytics.company.com:5432/analytics",
      "description": "Read-only analytics database"
    }
  },
  "default_connection": "production_pg"
}
```

## Usage

### Run as MCP Server (Stdio Transport)

```bash
# Using connection from config file
./db-mcp-server stdio --connection production_pg

# Using direct connection string
./db-mcp-server stdio --conn-string "postgres://user:pass@host/db"

# Read-only mode
./db-mcp-server stdio --connection dev_mysql --read-only
```

### Available Flags

- `--connection, -n`: Named connection from config file
- `--conn-string, -c`: Direct database connection string
- `--read-only, -r`: Enable read-only mode (SELECT queries only)
- `--toolsets, -t`: Toolsets to enable (default: `["db"]`)

## Current Tools

### execute_select
Execute SELECT queries and return formatted JSON results.

**Input:**
```json
{
  "query": "SELECT id, name, email FROM users LIMIT 5"
}
```

**Output:**
```json
{
  "results": "[{\"id\":1,\"name\":\"John\",\"email\":\"john@example.com\"}]"
}
```

## Planned Tools & Features

### 🚀 Next Phase - Database Schema Tools

#### 1. list_tables
List all tables in the database with metadata.

**Input:**
```json
{
  "schema": "public"  // optional
}
```

**Output:**
```json
{
  "tables": [
    {
      "name": "users",
      "schema": "public",
      "type": "table"
    }
  ]
}
```

#### 2. describe_table
Get detailed information about table structure, columns, and indexes.

**Input:**
```json
{
  "table_name": "users",
  "schema": "public"  // optional
}
```

**Output:**
```json
{
  "columns": [
    {
      "name": "id",
      "data_type": "integer",
      "is_nullable": false,
      "is_primary_key": true
    }
  ],
  "indexes": [
    {
      "name": "users_pkey",
      "columns": ["id"],
      "is_unique": true
    }
  ]
}
```

#### 3. get_db_info
Get general database information and statistics.

**Input:**
```json
{}
```

**Output:**
```json
{
  "database_name": "myapp",
  "version": "PostgreSQL 15.4",
  "schemas": ["public", "auth"],
  "table_count": 25
}
```

### 🔄 Phase 2 - Advanced Query Tools

#### 4. execute_query
Execute any SQL query (INSERT, UPDATE, DELETE, etc.) with proper permissions.

**Input:**
```json
{
  "query": "INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com')"
}
```

#### 5. explain_query
Get query execution plan for performance analysis.

**Input:**
```json
{
  "query": "SELECT * FROM users WHERE email = 'john@example.com'"
}
```

### 🛠️ Phase 3 - Connection Management

#### 6. list_connections
List all available named connections from config.

#### 7. switch_connection
Switch to a different database connection during the session.

#### 8. test_connection
Test connectivity to a database before executing queries.

### 📊 Phase 4 - Data Analysis Tools

#### 9. analyze_table
Get table statistics (row count, size, column stats).

#### 10. generate_sample_data
Generate sample data for testing purposes.

## Development Roadmap

### Phase 1: Configuration & Connection Management
- [ ] Implement config file loading from standard locations
- [ ] Add connection management with named connections
- [ ] Update CLI to support `--connection` flag
- [ ] Add connection validation and testing

### Phase 2: Schema Introspection
- [ ] Implement `list_tables` tool
- [ ] Implement `describe_table` tool
- [ ] Implement `get_db_info` tool
- [ ] Add PostgreSQL and MySQL specific schema queries

### Phase 3: Enhanced Query Support
- [ ] Implement `execute_query` tool for all SQL operations
- [ ] Add `explain_query` for query analysis
- [ ] Improve error handling and validation
- [ ] Add transaction support

### Phase 4: Advanced Features
- [ ] Connection pooling improvements
- [ ] Query caching mechanisms
- [ ] Performance monitoring
- [ ] Audit logging

## Architecture

```
db-mcp-server/
├── cmd/                    # CLI application entry point
├── internal/
│   ├── client/            # Database client wrapper
│   ├── config/            # Configuration management (TODO)
│   ├── state/             # Session state management
│   └── tools/             # MCP tool implementations
├── pkg/                   # Public types and schemas
└── connections.json       # Example configuration file
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/new-tool`
3. Implement your changes with tests
4. Submit a pull request

## Security Considerations

- **Read-only Mode**: Use `--read-only` flag for untrusted environments
- **Connection Strings**: Store sensitive credentials securely
- **Query Validation**: Built-in SQL injection protection
- **Timeouts**: All queries have configurable timeouts (default: 5s)

## Examples

### Connecting to Different Databases

```bash
# Connect to production PostgreSQL
./db-mcp-server stdio --connection production_pg

# Connect to development MySQL in read-only mode
./db-mcp-server stdio --connection dev_mysql --read-only

# Use direct connection string
./db-mcp-server stdio --conn-string "postgres://user:pass@localhost/test"
```

### Sample Queries

```sql
-- List all tables
SELECT table_name FROM information_schema.tables WHERE table_schema = 'public';

-- Get user data
SELECT id, name, email, created_at FROM users ORDER BY created_at DESC LIMIT 10;

-- Analyze table structure
DESCRIBE users;  -- MySQL
\d users         -- PostgreSQL
```

## License

MIT License - see LICENSE file for details.

## Support

For issues and feature requests, please use the GitHub issue tracker.