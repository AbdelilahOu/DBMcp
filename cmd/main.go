package main

import (
	_ "github.com/go-sql-driver/mysql" // Register MySQL driver
	_ "github.com/lib/pq"              // Register Postgres driver
)

func main() {
	// Bootstrap (Cobra handles CLI)
	Execute()
}
