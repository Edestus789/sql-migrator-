package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	MigratorOpt *Migrator
	LoggerOpt   *Logger
}

type Migrator struct {
	DSN       string
	Dir       string
	Type      string
	TableName string
}

type Logger struct {
	Level string
}

func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config

	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}
