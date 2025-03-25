package logger

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Logger interface {
	Fatal(msg string, v ...interface{})
	Error(msg string, v ...interface{})
	Warn(msg string, v ...interface{})
	Info(msg string, v ...interface{})
	Debug(msg string, v ...interface{})
}

type ZeroLogger struct{}

func New() *ZeroLogger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02 15:04:05"})

	level := getLevelFromEnv()
	zerolog.SetGlobalLevel(level)
	return &ZeroLogger{}
}

func getLevelFromEnv() zerolog.Level {
	level := os.Getenv("LOG_LEVEL")
	return getLevel(level)
}

func getLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "fatal":
		return zerolog.FatalLevel
	case "error":
		return zerolog.ErrorLevel
	case "warn":
		return zerolog.WarnLevel
	case "info":
		return zerolog.InfoLevel
	case "debug":
		return zerolog.DebugLevel
	default:
		return zerolog.InfoLevel
	}
}

func (l *ZeroLogger) Fatal(msg string, v ...interface{}) {
	log.Fatal().Msgf(msg, v...)
}

func (l *ZeroLogger) Error(msg string, v ...interface{}) {
	log.Error().Msgf(msg, v...)
}

func (l *ZeroLogger) Warn(msg string, v ...interface{}) {
	log.Warn().Msgf(msg, v...)
}

func (l *ZeroLogger) Info(msg string, v ...interface{}) {
	log.Info().Msgf(msg, v...)
}

func (l *ZeroLogger) Debug(msg string, v ...interface{}) {
	log.Debug().Msgf(msg, v...)
}
