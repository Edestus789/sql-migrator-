package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Edestus789/sql-migrator/app"
	"github.com/Edestus789/sql-migrator/config"
	"github.com/Edestus789/sql-migrator/logger"
	"github.com/Edestus789/sql-migrator/storage"
)

var (
	configPath    string
	path          string
	database      string
	migrationName string
	command       string
)

// var (
// 	ErrInvalidFlagNumber = errors.New("invalid flag number")

// 	configPath    string
// 	path          string
// 	database      string
// 	migrationName string
// 	command       string
// )

func init() {
	flag.StringVar(&configPath, "config", "config.yaml", "Path to config file")
	flag.StringVar(&path, "path", "", "Path to migrations file")
	flag.StringVar(&database, "dsn", "", "Database connection string")
	flag.StringVar(&migrationName, "name", "", "Migration name")
	flag.StringVar(&command, "command", "", "Command to run: create, up, down, redo, status, dbversion")
}

func main() {
	flag.Parse()

	config, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config file: %v\n", err)
		return
	}

	if path == "" {
		path = config.MigratorOpt.Dir
	} else {
		path = os.ExpandEnv(path)
	}

	if database == "" {
		database = config.MigratorOpt.DSN
	} else {
		database = os.ExpandEnv(database)
	}

	if migrationName == "" {
		migrationName = os.Getenv("NAME")
	}

	if path == "" || database == "" {
		fmt.Println("Path to migrations and database connection string must be provided.")
		return
	}

	if command == "" {
		fmt.Println("Command must be provided.")
		return
	}

	l := logger.New()
	db := storage.NewPostgresStorage(database, l)
	application := app.New(l, db)

	switch command {
	case "create":
		application.Create(migrationName, path, "sql")
	case "up":
		application.Up(path)
	case "down":
		application.Down(path)
	case "redo":
		application.Redo(path)
	case "status":
		application.Status()
	case "dbversion":
		application.DBVersion()
	default:
		fmt.Println("Invalid operation. Use one of the following: create, up, down, redo, status, dbversion.")
	}
}
