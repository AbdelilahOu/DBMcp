package client

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type DBClient struct {
	DB *sql.DB
}

func NewDBClient(connString string) (*DBClient, error) {
	driver := "postgres"
	if strings.HasPrefix(strings.ToLower(connString), "mysql") {
		driver = "mysql"
	}

	db, err := sql.Open(driver, connString)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", driver, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping %s: %w", driver, err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &DBClient{DB: db}, nil
}

func (c *DBClient) Close() error {
	return c.DB.Close()
}

func (c *DBClient) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return c.DB.QueryContext(ctx, query, args...)
}
