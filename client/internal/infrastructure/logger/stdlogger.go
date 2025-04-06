package logger

import (
	"log"
)

// StdLogger простой логгер на основе стандартного log пакета
type StdLogger struct {
	debugEnabled bool
}

// NewStdLogger создает новый логгер
func NewStdLogger(debugEnabled bool) *StdLogger {
	return &StdLogger{
		debugEnabled: debugEnabled,
	}
}

// Info логирует информационное сообщение
func (l *StdLogger) Info(msg string, args ...interface{}) {
	log.Printf(msg, args...)
}

// Error логирует сообщение об ошибке
func (l *StdLogger) Error(msg string, args ...interface{}) {
	log.Printf("ОШИБКА: "+msg, args...)
}

// Debug логирует отладочное сообщение
func (l *StdLogger) Debug(msg string, args ...interface{}) {
	if l.debugEnabled {
		log.Printf("DEBUG: "+msg, args...)
	}
}
