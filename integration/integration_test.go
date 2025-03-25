//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/Edestus789/sql-migrator/app"
	"github.com/Edestus789/sql-migrator/logger"
	"github.com/Edestus789/sql-migrator/storage"
	_ "github.com/lib/pq"
)

const (
	dbUser     = "user"
	dbPassword = "password"
	dbName     = "test_db"
	dbHost     = "localhost"
	dbPort     = "5432"
)

func getDBConnection() *sql.DB {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func setup() *storage.PostgresStorage {
	logger := logger.New()
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	storage := storage.New(connStr, logger)
	ctx := context.Background()
	if err := storage.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	return storage
}

func teardown(storage *storage.PostgresStorage) {
	ctx := context.Background()
	if err := storage.DeleteMigrations(ctx); err != nil {
		log.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		log.Fatal(err)
	}
}

func TestMigrations(t *testing.T) {
	db := getDBConnection()
	defer db.Close()

	storage := setup()
	defer teardown(storage)

	logger := logger.New()
	application := app.New(logger, storage)

	migrationDir := "../migrations"
	os.MkdirAll(migrationDir, os.ModePerm)

	application.Create("create_users", migrationDir, "sql")

	application.Up(migrationDir)

	var tableName string
	err := db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_name = 'users'").Scan(&tableName)
	if err != nil {
		t.Fatalf("Expected table 'users' to be created, but got error: %v", err)
	}
	if tableName != "users" {
		t.Fatalf("Expected table 'users', but got: %s", tableName)
	}

	application.Down(migrationDir)

	err = db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_name = 'users'").Scan(&tableName)
	if err == nil || tableName == "users" {
		t.Fatalf("Expected table 'users' to be dropped, but it still exists")
	}

	os.Remove(fmt.Sprintf("%s/00001_%s_up.sql", migrationDir, "create_users"))
	os.Remove(fmt.Sprintf("%s/00001_%s_down.sql", migrationDir, "create_users"))
}
