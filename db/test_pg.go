package main

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	connStr := "postgres://pguser:pgpass@localhost:5432/pgdb?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Connection error:", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal("Ping failed:", err)
	}
	log.Println("✅ Successfully connected to PostgreSQL!")
}
