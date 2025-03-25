package storage

import (
	"context"
	"time"
)

type IMigration interface {
	GetName() string
	GetStatus() string
	GetVersion() int
	GetStatusChangeTime() time.Time

	SetName(name string)
	SetStatus(status string)
	SetVersion(version int)
	SetStatusChangeTime(statusChangeTime time.Time)
}

type Migration struct {
	Name             string
	Version          int
	Status           string
	StatusChangeTime time.Time
	Up               string
	Down             string
	UpGo             func(ctx context.Context) error
	DownGo           func(ctx context.Context) error
}

func CreateMigration(name, status string, version int, statusChangeTime time.Time) IMigration {
	return &Migration{
		Name:             name,
		Status:           status,
		Version:          version,
		StatusChangeTime: statusChangeTime,
	}
}

func (m *Migration) GetName() string {
	return m.Name
}

func (m *Migration) GetStatus() string {
	return m.Status
}

func (m *Migration) GetVersion() int {
	return m.Version
}

func (m *Migration) GetStatusChangeTime() time.Time {
	return m.StatusChangeTime
}

func (m *Migration) SetName(name string) {
	m.Name = name
}

func (m *Migration) SetStatus(status string) {
	m.Status = status
}

func (m *Migration) SetVersion(version int) {
	m.Version = version
}

func (m *Migration) SetStatusChangeTime(statusChangeTime time.Time) {
	m.StatusChangeTime = statusChangeTime
}
