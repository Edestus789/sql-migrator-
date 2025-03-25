package storage

import (
	"context"
	"errors"
	"time"

	"github.com/Edestus789/sql-migrator/logger"
	"github.com/jackc/pgx/v4/pgxpool"
)

// advisoryLockID — это идентификатор, используемый для создания уникальной блокировки.
// Он должен быть уникальным для приложения и не пересекаться с другими существующими возможными блокировками в бд.
const advisoryLockID = 123456

type SQLStorage interface {
	Connect(ctx context.Context) error
	Close() error
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
	InsertMigration(ctx context.Context, migration IMigration) error
	Migrate(ctx context.Context, sql string) error
	SelectMigrations(ctx context.Context) ([]IMigration, error)
	SelectLastMigrationByStatus(ctx context.Context, status string) (IMigration, error)
	DeleteMigrations(ctx context.Context) error
}

const (
	StatusProcess      = "process"
	StatusSuccess      = "success"
	StatusError        = "error"
	StatusCancellation = "cancellation"
	StatusCancel       = "cancel"
)

type PostgresStorage struct {
	connString string
	pool       *pgxpool.Pool
	logger     logger.Logger
}

var (
	ErrUnexpectedStatus  = errors.New("unexpected status")
	ErrMigrationNotFound = errors.New("processes not found")
)

func NewPostgresStorage(connString string, logger logger.Logger) *PostgresStorage {
	return &PostgresStorage{
		connString: connString,
		logger:     logger,
	}
}

func (storage *PostgresStorage) Connect(ctx context.Context) error {
	storage.logger.Info("Connecting to the database")

	pool, err := pgxpool.Connect(ctx, storage.connString)
	if err != nil {
		storage.logger.Error("Failed to connect to the database: %v", err)
		return err
	}

	sql := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			Version INTEGER PRIMARY KEY,
			Name CHARACTER VARYING(100),
			Status CHARACTER VARYING(20),
			StatusChangeTime TIMESTAMP
		);`

	_, err = pool.Exec(ctx, sql)
	if err != nil {
		storage.logger.Error("Failed to create schema_migrations table: %v", err)
		pool.Close()
		return err
	}

	storage.pool = pool
	storage.logger.Info("Connected to the database and " +
		"ensured schema_migrations table exists")
	return nil
}

func (storage *PostgresStorage) Close() error {
	storage.logger.Info("Closing database connection pool")

	if storage.pool != nil {
		storage.pool.Close()
		storage.logger.Info("Database connection pool closed")
	}
	return nil
}

func (storage *PostgresStorage) Lock(ctx context.Context) error {
	storage.logger.Info("Acquiring advisory lock")
	_, err := storage.pool.Exec(ctx,
		"SELECT pg_advisory_lock($1);",
		advisoryLockID)
	if err != nil {
		storage.logger.Error("Failed to acquire advisory lock: %v", err)
	}
	return err
}

func (storage *PostgresStorage) Unlock(ctx context.Context) error {
	storage.logger.Info("Releasing advisory lock")
	_, err := storage.pool.Exec(ctx,
		"SELECT pg_advisory_unlock($1);",
		advisoryLockID)
	if err != nil {
		storage.logger.Error("Failed to release advisory lock: %v", err)
	}
	return err
}

func (storage *PostgresStorage) DeleteMigrations(ctx context.Context) error {
	storage.logger.Info("Deleting all migrations from schema_migrations table")
	_, err := storage.pool.Exec(ctx, "TRUNCATE schema_migrations;")
	if err != nil {
		storage.logger.Error("Failed to delete migrations: %v", err)
	}
	return err
}

func (storage *PostgresStorage) SelectMigrations(ctx context.Context) ([]IMigration, error) {
	storage.logger.Info("Selecting all migrations from schema_migrations table")
	sql := `SELECT Name, Status, Version, StatusChangeTime FROM schema_migrations ORDER BY Version DESC;`

	rows, err := storage.pool.Query(ctx, sql)
	if err != nil {
		storage.logger.Error("Failed to select migrations: %v", err)
		return nil, err
	}
	defer rows.Close()

	var migrations []IMigration
	for rows.Next() {
		var (
			name             string
			version          int
			status           string
			statusChangeTime time.Time
		)

		err = rows.Scan(&name, &status, &version, &statusChangeTime)
		if err != nil {
			storage.logger.Error("Failed to scan migration row: %v", err)
			return nil, err
		}

		migrations = append(migrations, CreateMigration(name, status, version, statusChangeTime))
	}

	if len(migrations) == 0 {
		storage.logger.Warn("No migrations found")
		return nil, ErrMigrationNotFound
	}

	return migrations, nil
}

func (storage *PostgresStorage) SelectLastMigrationByStatus(ctx context.Context, status string) (IMigration, error) {
	storage.logger.Info("Выбор последней миграции со статусом: %s", status)

	switch status {
	case StatusSuccess, StatusError, StatusProcess, StatusCancellation, StatusCancel:
	default:
		storage.logger.Error("Неожиданный статус: %s", status)
		return nil, ErrUnexpectedStatus
	}

	sql := `SELECT Name, Status, Version, StatusChangeTime 
        FROM schema_migrations 
        WHERE Status = $1 
        ORDER BY Version DESC 
        LIMIT 1;`

	row := storage.pool.QueryRow(ctx, sql, status)

	var (
		name             string
		version          int
		statusStr        string
		statusChangeTime time.Time
	)

	err := row.Scan(&name, &statusStr, &version, &statusChangeTime)
	if err != nil {
		// if err == pgx.ErrNoRows {
		// 	storage.logger.Warn("Миграция со статусом %s не найдена", status)
		// 	return nil, ErrMigrationNotFound
		// }
		storage.logger.Error(
			"Ошибка при получении последней миграции со статусом %s: %v",
			status,
			err,
		)
		return nil, err
	}

	return CreateMigration(name, statusStr, version, statusChangeTime), nil
}

func (storage *PostgresStorage) InsertMigration(ctx context.Context, migration IMigration) error {
	storage.logger.Info("Inserting/updating migration: %s", migration.GetName())

	sql := `
		DO $$ 
		BEGIN
			IF EXISTS (SELECT 1 FROM schema_migrations WHERE Version = $1 AND Name = $2) THEN
				UPDATE schema_migrations 
				SET Status = $3, StatusChangeTime = $4 
				WHERE Version = $1 AND Name = $2;
			ELSE
				INSERT INTO schema_migrations 
					(Version, Name, Status, StatusChangeTime)
				VALUES 
					($1, $2, $3, $4);
			END IF;
		END $$;`

	_, err := storage.pool.Exec(ctx, sql, migration.GetVersion(), migration.GetName(), migration.GetStatus(), migration.GetStatusChangeTime())
	if err != nil {
		storage.logger.Error("Failed to insert/update migration: %v", err)
	}
	return err
}

func (storage *PostgresStorage) Migrate(ctx context.Context, sql string) error {
	storage.logger.Info("Executing migration SQL")
	_, err := storage.pool.Exec(ctx, sql)
	if err != nil {
		storage.logger.Error("Failed to execute migration SQL: %v", err)
	}
	return err
}
