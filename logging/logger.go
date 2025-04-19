// Package logging предоставляет стандартизированный интерфейс для логирования
package logging

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"io"
	"log"
	"os"
	"strings"
)

// LogLevel уровень логирования
type LogLevel string

const (
	// DEBUG уровень логирования для отладки
	DEBUG LogLevel = "debug"
	// INFO уровень логирования для информационных сообщений
	INFO LogLevel = "info"
	// WARNING уровень логирования для предупреждений
	WARNING LogLevel = "warning"
	// ERROR уровень логирования для ошибок
	ERROR LogLevel = "error"
	// FATAL уровень логирования для критических ошибок
	FATAL LogLevel = "fatal"
)

// Logger представляет интерфейс логгера
type Logger interface {
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Error(format string, v ...interface{})
	Fatal(format string, v ...interface{})

	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
	WithError(err error) Logger
	WithContext(ctx context.Context) Logger
	WithRequestID(requestID string) Logger
}

type requestIDKey struct{}

// ExtractRequestID извлекает ID запроса из контекста
func ExtractRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if reqID, ok := ctx.Value(requestIDKey{}).(string); ok {
		return reqID
	}
	return ""
}

// ContextWithRequestID добавляет ID запроса в контекст
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		requestID = uuid.New().String()
	}
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// GenerateRequestID генерирует новый ID запроса
func GenerateRequestID() string {
	return uuid.New().String()
}

// DefaultLogger реализует интерфейс Logger с базовой функциональностью
type DefaultLogger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	fatalLogger *log.Logger
	fields      map[string]interface{}
}

// Создание нового логгера с указанным уровнем и полями
func NewLogger() Logger {
	// Получаем уровень логирования из переменной окружения
	levelStr := strings.ToLower(os.Getenv("LOG_LEVEL"))
	if levelStr == "" {
		levelStr = "info"
	}

	level := LogLevel(levelStr)

	// Определяем, какие логгеры будут активны
	var (
		debugOutput io.Writer = io.Discard
		infoOutput  io.Writer = io.Discard
		warnOutput  io.Writer = io.Discard
		errorOutput io.Writer = os.Stderr
		fatalOutput io.Writer = os.Stderr
	)

	switch level {
	case DEBUG:
		debugOutput = os.Stdout
		infoOutput = os.Stdout
		warnOutput = os.Stdout
	case INFO:
		infoOutput = os.Stdout
		warnOutput = os.Stdout
	case WARNING:
		warnOutput = os.Stdout
	case ERROR, FATAL:
		// По умолчанию только error и fatal
	}

	// Создаем логгеры для каждого уровня
	debugLogger := log.New(debugOutput, "[DEBUG] ", log.Ldate|log.Ltime|log.Lshortfile)
	infoLogger := log.New(infoOutput, "[INFO] ", log.Ldate|log.Ltime)
	warnLogger := log.New(warnOutput, "[WARN] ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger := log.New(errorOutput, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile)
	fatalLogger := log.New(fatalOutput, "[FATAL] ", log.Ldate|log.Ltime|log.Lshortfile)

	// Переопределяем стандартный логгер для использования в других пакетах
	log.SetOutput(infoOutput)
	log.SetPrefix("[INFO] ")
	log.SetFlags(log.Ldate | log.Ltime)

	return &DefaultLogger{
		debugLogger: debugLogger,
		infoLogger:  infoLogger,
		warnLogger:  warnLogger,
		errorLogger: errorLogger,
		fatalLogger: fatalLogger,
		fields:      make(map[string]interface{}),
	}
}

// formatMessage форматирует сообщение с учетом полей
func (l *DefaultLogger) formatMessage(format string, v ...interface{}) string {
	message := fmt.Sprintf(format, v...)

	if len(l.fields) == 0 {
		return message
	}

	fieldsStr := "{"
	first := true
	for k, v := range l.fields {
		if !first {
			fieldsStr += ", "
		}
		fieldsStr += fmt.Sprintf("%s: %v", k, v)
		first = false
	}
	fieldsStr += "}"

	return fmt.Sprintf("%s %s", message, fieldsStr)
}

// Debug логирует сообщение на уровне DEBUG
func (l *DefaultLogger) Debug(format string, v ...interface{}) {
	l.debugLogger.Output(2, l.formatMessage(format, v...))
}

// Info логирует сообщение на уровне INFO
func (l *DefaultLogger) Info(format string, v ...interface{}) {
	l.infoLogger.Output(2, l.formatMessage(format, v...))
}

// Warn логирует сообщение на уровне WARNING
func (l *DefaultLogger) Warn(format string, v ...interface{}) {
	l.warnLogger.Output(2, l.formatMessage(format, v...))
}

// Error логирует сообщение на уровне ERROR
func (l *DefaultLogger) Error(format string, v ...interface{}) {
	l.errorLogger.Output(2, l.formatMessage(format, v...))
}

// Fatal логирует сообщение на уровне FATAL и завершает программу
func (l *DefaultLogger) Fatal(format string, v ...interface{}) {
	l.fatalLogger.Output(2, l.formatMessage(format, v...))
	os.Exit(1)
}

// WithField добавляет поле в логгер
func (l *DefaultLogger) WithField(key string, value interface{}) Logger {
	newLogger := &DefaultLogger{
		debugLogger: l.debugLogger,
		infoLogger:  l.infoLogger,
		warnLogger:  l.warnLogger,
		errorLogger: l.errorLogger,
		fatalLogger: l.fatalLogger,
		fields:      make(map[string]interface{}),
	}

	// Копируем существующие поля
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Добавляем новое поле
	newLogger.fields[key] = value

	return newLogger
}

// WithFields добавляет несколько полей в логгер
func (l *DefaultLogger) WithFields(fields map[string]interface{}) Logger {
	newLogger := &DefaultLogger{
		debugLogger: l.debugLogger,
		infoLogger:  l.infoLogger,
		warnLogger:  l.warnLogger,
		errorLogger: l.errorLogger,
		fatalLogger: l.fatalLogger,
		fields:      make(map[string]interface{}),
	}

	// Копируем существующие поля
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Добавляем новые поля
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// WithError добавляет ошибку в логгер
func (l *DefaultLogger) WithError(err error) Logger {
	return l.WithField("error", err.Error())
}

// WithContext добавляет контекст в логгер
func (l *DefaultLogger) WithContext(ctx context.Context) Logger {
	logger := l

	// Извлекаем request_id из контекста, если есть
	if requestID := ExtractRequestID(ctx); requestID != "" {
		logger = logger.WithField("request_id", requestID).(*DefaultLogger)
	}

	// Можно добавить извлечение других данных из контекста

	return logger
}

// WithRequestID добавляет ID запроса в логгер
func (l *DefaultLogger) WithRequestID(requestID string) Logger {
	return l.WithField("request_id", requestID)
}
