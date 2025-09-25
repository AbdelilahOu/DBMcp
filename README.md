# Database MCP Server

A Model Context Protocol (MCP) server that provides comprehensive database connectivity and query capabilities for Claude and other AI assistants. This server enables seamless interaction with databases through a standardized interface, supporting multiple database types and connection management.

## What is this project?

This MCP server bridges the gap between AI assistants and database systems, allowing Claude to:

- **Connect to multiple databases** - PostgreSQL, MySQL, and other SQL databases
- **Execute queries safely** - With built-in validation and optional read-only modes
- **Explore database schemas** - Inspect tables, columns, indexes, and relationships
- **Analyze data** - Get table statistics and query performance insights
- **Manage connections** - Switch between different database environments seamlessly

## Key Features

- 🔗 **Multi-Database Support** - Connect to PostgreSQL, MySQL, and other SQL databases
- 🛡️ **Security First** - Read-only mode, query validation, and secure credential handling
- 📊 **Rich Schema Inspection** - Detailed table descriptions, column metadata, and index information
- ⚡ **Performance Analysis** - Query execution plans and table statistics
- 🔧 **Flexible Connection Management** - Named connections with easy switching
- 🎯 **AI-Optimized** - Designed specifically for AI assistant workflows

## Available Tools

The server provides comprehensive database interaction capabilities:

### Query Execution
- `execute_select` - Run SELECT queries with formatted JSON results
- `execute_query` - Execute any SQL operation (INSERT, UPDATE, DELETE, etc.)

### Schema Exploration
- `describe_table` - Get detailed table structure, columns, and indexes
- `list_tables` - Browse all available tables with metadata
- `get_db_info` - Access general database information and statistics

### Performance & Analysis
- `explain_query` - Analyze query execution plans for optimization
- `analyze_table` - Retrieve table statistics and performance metrics

### Connection Management
- `list_connections` - View all configured database connections
- `switch_connection` - Change active database connection during sessions
- `test_connection` - Verify database connectivity before operations

## Use Cases

This MCP server is perfect for:

- **Database Administration** - Schema exploration and maintenance tasks
- **Data Analysis** - Querying and analyzing data with AI assistance
- **Development Support** - Understanding database structures and relationships
- **Performance Tuning** - Analyzing query plans and optimizing database performance
- **Documentation** - Generating database documentation and schemas
- **Migration Planning** - Understanding existing database structures

## Security & Safety

Built with security as a priority:
- **Read-only mode** for safe exploration
- **Query validation** to prevent harmful operations
- **Connection timeouts** to prevent resource exhaustion
- **Secure credential management** through configuration files

Perfect for teams who want to leverage AI assistance for database work while maintaining security and control over their data.
