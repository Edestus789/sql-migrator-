package processes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Edestus789/sql-migrator/logger"
	"github.com/Edestus789/sql-migrator/storage"
)

// Интерфейс IMigration определяет методы для работы с миграциями.
type IMigration interface {
	Connect(context.Context) error
	Close(context.Context) error
	Create(name, up, down string, upGo, downGo func(ctx context.Context) error)
	Up(context.Context) error
	Down(context.Context) error
	Redo(context.Context) error
	Status(context.Context) error
	DBVersion(context.Context) error
}

// Структура Migrator реализует интерфейс IMigration.
type Migrator struct {
	logger     logger.Logger
	storage    storage.SQLStorage
	migrations []storage.Migration
}

// Определение ошибок для обработки различных ситуаций.
var (
	ErrMigrationUp                = errors.New("ошибка выполнения миграции вверх")
	ErrMigrationDown              = errors.New("ошибка выполнения миграции вниз")
	ErrMigrationRedo              = errors.New("ошибка выполнения повторной миграции")
	ErrGetStatus                  = errors.New("ошибка получения статуса БД")
	ErrGetVersion                 = errors.New("ошибка получения версии БД")
	ErrUnexpectedMigrationVersion = errors.New("неожиданная версия миграции")
)

// Конструктор для создания нового объекта Migrator.
func New(connString storage.SQLStorage, logger logger.Logger) *Migrator {
	return &Migrator{
		storage:    connString,
		logger:     logger,
		migrations: make([]storage.Migration, 0),
	}
}

// Метод для подключения к базе данных.
func (m *Migrator) Connect(ctx context.Context) error {
	m.logger.Info("Подключение к базе данных")

	if err := m.storage.Connect(ctx); err != nil {
		m.logger.Error("Ошибка при подключении: %v", err)
		return err
	}

	m.logger.Info("Подключение к базе данных успешно")
	return nil
}

// Метод для закрытия подключения к базе данных.
func (m *Migrator) Close(ctx context.Context) error {
	m.logger.Info("Закрытие подключения к базе данных")

	if err := m.storage.Close(); err != nil {
		m.logger.Error("Ошибка при закрытии: %v", err)
		return err
	}

	m.logger.Info("Подключение к базе данных закрыто")
	return nil
}

// Метод для создания миграции.
func (m *Migrator) Create(name, up, down string, upGo, downGo func(ctx context.Context) error) {
	m.logger.Info("Создание миграции: %s", name)
	m.migrations = append(m.migrations, storage.Migration{
		Status:  "success",
		Version: len(m.migrations) + 1,
		Name:    name,
		Up:      up,
		Down:    down,
		UpGo:    upGo,
		DownGo:  downGo,
	})
	m.logger.Info("Миграция %s создана", name)
}

// Метод для выполнения миграций вверх.
func (m *Migrator) Up(ctx context.Context) error {
	m.logger.Info("Начало выполнения миграций")

	if err := m.storage.Lock(ctx); err != nil {
		m.logger.Error("Ошибка при блокировке: %v", err)
		return err
	}
	defer func(storage storage.SQLStorage, ctx context.Context) {
		err := storage.Unlock(ctx)
		if err != nil {
			m.logger.Error("Ошибка при разблокировке: %v", err)
		}
	}(m.storage, ctx)

	lastVersion := 0
	lastMigration, err := m.storage.SelectLastMigrationByStatus(ctx, storage.StatusSuccess)
	if err == nil {
		lastVersion = lastMigration.GetVersion()
	} else if !errors.Is(err, storage.ErrMigrationNotFound) {
		m.logger.Error("1Ошибка при получении последней успешной миграции: %v", err)
		return err
	}

	if lastMigration != nil && lastMigration.GetVersion()-1 > len(m.migrations) {
		m.logger.Error("Ошибка: %v", ErrUnexpectedMigrationVersion)
		return ErrUnexpectedMigrationVersion
	}

	for i := lastVersion; i < len(m.migrations); i++ {
		err = m.upMigration(ctx, &m.migrations[i], m.migrations[i].Up, m.migrations[i].UpGo)
		if err != nil {
			m.logger.Error("Ошибка при выполнении миграции вверх: %v", err)
			return ErrMigrationUp
		}
	}

	m.logger.Info("Миграции успешно выполнены")
	return nil
}

func (m *Migrator) Down(ctx context.Context) error {
	m.logger.Info("Начало выполнения отката миграций")

	if err := m.storage.Lock(ctx); err != nil {
		m.logger.Error("Ошибка при блокировке: %v", err)
		return err
	}
	defer func(storage storage.SQLStorage, ctx context.Context) {
		err := storage.Unlock(ctx)
		if err != nil {
			m.logger.Error("Ошибка при разблокировке: %v", err)
		}
	}(m.storage, ctx)

	lastMigration, err := m.storage.SelectLastMigrationByStatus(ctx, storage.StatusSuccess)
	if err != nil {
		if errors.Is(err, storage.ErrMigrationNotFound) {
			m.logger.Warn("Нет успешных миграций для отката")
			return nil
		}
		m.logger.Error("Ошибка при получении "+
			"последней успешной миграции: %v", err)
		return err
	}

	if lastMigration.GetVersion() > len(m.migrations) {
		m.logger.Error("Ошибка: %v", ErrUnexpectedMigrationVersion)
		return ErrUnexpectedMigrationVersion
	}

	downMigrationIndex := lastMigration.GetVersion() - 1
	err = m.downMigration(ctx, &m.migrations[downMigrationIndex], m.migrations[downMigrationIndex].Down, m.migrations[downMigrationIndex].DownGo)
	if err != nil {
		m.logger.Error("Ошибка при выполнении отката миграции: %v", err)
		return ErrMigrationDown
	}

	m.logger.Info("Откат миграций успешно выполнен")
	return nil
}

// Вспомогательный метод для выполнения миграции.
func (m *Migrator) executeMigration(ctx context.Context, migration storage.IMigration, sql string, goFunc func(ctx context.Context) error, processStatus, successStatus, errorStatus string) error {
	migration.SetStatus(processStatus)
	migration.SetStatusChangeTime(time.Now())

	if err := m.storage.InsertMigration(ctx, migration); err != nil {
		m.logger.Error("Ошибка при вставке миграции: %v", err)
		return err
	}

	if goFunc != nil {
		if err := goFunc(ctx); err != nil {
			migration.SetStatus(errorStatus)
			migration.SetStatusChangeTime(time.Now())
			err := m.storage.InsertMigration(ctx, migration)
			m.logger.Error("Ошибка при выполнении Go-миграции: %v", err)
			return err
		}
	} else if sql != "" {
		if err := m.storage.Migrate(ctx, sql); err != nil {
			migration.SetStatus(errorStatus)
			migration.SetStatusChangeTime(time.Now())
			err := m.storage.InsertMigration(ctx, migration)
			m.logger.Error("Ошибка при выполнении SQL-миграции: %v", err)
			return err
		}
	}

	migration.SetStatus(successStatus)
	migration.SetStatusChangeTime(time.Now())
	if err := m.storage.InsertMigration(ctx, migration); err != nil {
		m.logger.Error("Ошибка при вставке миграции: %v", err)
		return err
	}

	m.logger.Info("Миграция %s до версии %d успешно применена", migration.GetName(), migration.GetVersion())
	return nil
}

// Метод для выполнения миграции вверх.
func (m *Migrator) upMigration(ctx context.Context, migration storage.IMigration, sql string, upGo func(ctx context.Context) error) error {
	return m.executeMigration(ctx, migration, sql, upGo, storage.StatusProcess, storage.StatusSuccess, storage.StatusError)
}

// Метод для выполнения миграции вниз.
func (m *Migrator) downMigration(ctx context.Context, migration storage.IMigration, sql string, downGo func(ctx context.Context) error) error {
	return m.executeMigration(ctx, migration, sql, downGo, storage.StatusCancellation, storage.StatusCancel, storage.StatusError)
}

// Метод для выполнения повторной миграции.
func (m *Migrator) Redo(ctx context.Context) error {
	m.logger.Info("Начало выполнения повторной миграции")

	err := m.Down(ctx)
	if err != nil {
		m.logger.Error("Ошибка при откате миграции: %v", err)
		return err
	}

	lastVersion := 0
	lastMigration, err := m.storage.SelectLastMigrationByStatus(ctx, storage.StatusSuccess)
	if err == nil {
		lastVersion = lastMigration.GetVersion()
	} else if !errors.Is(err, storage.ErrMigrationNotFound) {
		m.logger.Error("Ошибка при получении последней успешной миграции: %v", err)
		return err
	}

	if lastMigration != nil && lastMigration.GetVersion()-1 > len(m.migrations) {
		m.logger.Error("Ошибка: %v", ErrUnexpectedMigrationVersion)
		return ErrUnexpectedMigrationVersion
	}

	err = m.upMigration(ctx, &m.migrations[lastVersion], m.migrations[lastVersion].Up, m.migrations[lastVersion].UpGo)
	if err != nil {
		m.logger.Error("Ошибка при повторной миграции: %v", err)
		return ErrMigrationRedo
	}

	m.logger.Info("Повторная миграция успешно выполнена")
	return nil
}

// Метод для получения статуса миграций.
func (m *Migrator) Status(ctx context.Context) error {
	migrations, err := m.storage.SelectMigrations(ctx)
	if err != nil {
		m.logger.Error("Ошибка при получении статуса: %v", err)
		return ErrGetStatus
	}

	border := "._____________________._____________________._____________________."
	header := fmt.Sprintf("| %-19s | %-19s | %-19s |",
		"Название", "Статус", "Время")
	m.logger.Info(border)
	m.logger.Info(header)

	for _, migr := range migrations {
		formatMigration := fmt.Sprintf("| %-19s | %-19s | %s |",
			migr.GetName(),
			migr.GetStatus(),
			migr.GetStatusChangeTime().Format("2006-01-02 15:04:05"))

		m.logger.Info(formatMigration)
	}

	m.logger.Info(border)
	return nil
}

// Метод для получения текущей версии базы данных.
func (m *Migrator) DBVersion(ctx context.Context) error {
	lastVersion := 0

	lastMigration, err := m.storage.SelectLastMigrationByStatus(ctx, storage.StatusSuccess)
	if err == nil {
		lastVersion = lastMigration.GetVersion()
	} else if !errors.Is(err, storage.ErrMigrationNotFound) {
		m.logger.Error("Ошибка при получении версии БД: %v", err)
		return ErrGetVersion
	}

	m.logger.Info("Версия: %d", lastVersion)
	return nil
}
