package db

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
	"github.com/workshop/tapas-backend/chaos"
)

func Connect() (*sql.DB, error) {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/tapas?sslmode=disable"
	}
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	// CHAOS: single connection forces all queries to serialize, amplifying the
	// n+1 query pattern into visible connection-wait latency.
	if chaos.Enabled() {
		conn.SetMaxOpenConns(1)
	}
	return conn, nil
}
