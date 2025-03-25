package app

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Edestus789/sql-migrator/logger"
	"github.com/Edestus789/sql-migrator/storage"
	"github.com/stretchr/testify/assert"
)

func TestCreateMigrationFiles(t *testing.T) {
	logger := logger.New()
	mockStorage := storage.NewMockSQLStorage()
	app := New(logger, mockStorage)

	migrationDir := "../migrations"
	migrationName := "create_users"

	// Создаем директорию миграций
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Удаляем директорию после теста
	t.Cleanup(func() {
		os.RemoveAll(migrationDir)
	})

	app.Create(migrationName, migrationDir, "sql")

	upFile := fmt.Sprintf("%s/00001_%s_up.sql", migrationDir, migrationName)
	downFile := fmt.Sprintf("%s/00001_%s_down.sql", migrationDir, migrationName)
	assert.FileExists(t, upFile, "Expected Up migration file to be created")
	assert.FileExists(t, downFile, "Expected Down migration file to be created")

	os.Remove(upFile)
	os.Remove(downFile)
}

func TestUpDownMigration(t *testing.T) {
	logger := logger.New()
	mockStorage := storage.NewMockSQLStorage()
	app := New(logger, mockStorage)

	migrationDir := "../migrations"
	migrationName := "create_users"

	// Создаем директорию миграций
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Удаляем директорию после теста
	t.Cleanup(func() {
		os.RemoveAll(migrationDir)
	})

	// Добавляем миграцию со статусом "success"
	successMigration := storage.CreateMigration(migrationName, storage.StatusSuccess, 1, time.Now())
	if err := mockStorage.InsertMigration(context.Background(), successMigration); err != nil {
		t.Fatalf("Failed to insert migration: %v", err)
	}

	// Создаем миграцию
	app.Create(migrationName, migrationDir, "sql")
	// Выполняем миграцию вверх
	app.Up(migrationDir)

	// Выполняем откат миграции
	app.Down(migrationDir)

	// Проверяем, что миграция была откатана
	migrations, _ := mockStorage.SelectMigrations(context.Background())
	assert.Equal(t, 1, len(migrations), "Expected one migration after rollback")
	assert.Equal(t, storage.StatusCancel, migrations[0].GetStatus(), "Expected migration status to be 'canceled'")

	// Удаляем файлы миграции
	files, _ := os.ReadDir(migrationDir)
	latestVersion := getLastVersion(files, logger)
	os.Remove(fmt.Sprintf("%s/%05d_%s_up.sql", migrationDir, latestVersion, migrationName))
	os.Remove(fmt.Sprintf("%s/%05d_%s_down.sql", migrationDir, latestVersion, migrationName))
}
