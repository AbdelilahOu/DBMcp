package main

import (
	"log"

	"github.com/AbdelilahOu/DBMcp/internal/tools"
	_ "github.com/go-sql-driver/mysql" // Register MySQL driver
	_ "github.com/lib/pq"              // Register Postgres driver
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Bootstrap (Cobra handles CLI)
	if err := Execute(); err != nil {
		log.Fatal(err)
	}
}
