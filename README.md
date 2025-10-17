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

- **Multi-Database Support** - Connect to PostgreSQL, MySQL, and other SQL databases
- **Security First** - Read-only mode, query validation, and secure credential handling
- **Rich Schema Inspection** - Detailed table descriptions, column metadata, and index information
- **Performance Analysis** - Query execution plans and table statistics
- **Flexible Connection Management** - Named connections with easy switching
- **AI-Optimized** - Designed specifically for AI assistant workflows

## Available Tools

The server provides comprehensive database interaction capabilities through **28 specialized tools**:

### Connection Management
- `list_connections` - View all configured database connections
- `switch_connection` - Change active database connection during sessions
- `test_connection` - Verify database connectivity before operations

### Database Discovery & Metadata
- `get_db_info` - Access general database information and statistics
- `list_tables` - Browse all available tables with metadata
- `describe_table` - Get detailed table structure, columns, and indexes
- `analyze_table` - Retrieve table statistics and performance metrics

### Query Execution
- `select_query` - Execute SELECT queries and retrieve data
- `execute_query` - Execute data modification (INSERT, UPDATE, DELETE) and DDL statements
- `show_query` - Execute SHOW commands for database settings

### Foreign Keys & Relationships
- `list_foreign_keys` - List all foreign key constraints with referenced tables and actions
- `get_table_relationships` - Get incoming and outgoing relationships for a table

### Views
- `list_views` - List all views (separate from tables)
- `get_view_definition` - Get SQL definition of a specific view
- `list_materialized_views` - List materialized views (PostgreSQL only)

### Sequences (PostgreSQL)
- `list_sequences` - List all sequences in the database
- `get_sequence_info` - Get detailed sequence information (current value, increment, etc.)

### Triggers
- `list_triggers` - List all triggers with events and timing information
- `get_trigger_definition` - Get complete trigger SQL definition

### Functions & Stored Procedures
- `list_functions` - List all user-defined functions and stored procedures
- `get_function_definition` - Get complete function source code

### Enums (PostgreSQL)
- `list_enums` - List all enum types in the database
- `get_enum_values` - Get all possible values for a specific enum type

### Column Search
- `find_column` - Search for columns by name across all tables (supports partial matching)

### Constraints
- `list_constraints` - List all constraints (PRIMARY KEY, FOREIGN KEY, UNIQUE, CHECK)

### Utilities
- `generate_id` - Generate unique identifiers (UUID v1-v7, CUID, CUID2)

## Use Cases

This MCP server is perfect for:

- **Database Administration** - Schema exploration and maintenance tasks
- **Data Analysis** - Querying and analyzing data with AI assistance
- **Development Support** - Understanding database structures and relationships
- **Performance Tuning** - Analyzing query plans and optimizing database performance
- **Documentation** - Generating database documentation and schemas
- **Migration Planning** - Understanding existing database structures
