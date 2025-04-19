// Package database предоставляет унифицированный интерфейс для работы с базами данных
package database

import (
	"fmt"
	"time"

	"github.com/vladzorgan/common/logging"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	goormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Database представляет соединение с базой данных
type Database struct {
	db     *gorm.DB
	logger logging.Logger
}

// DatabaseOptions содержит опции для создания соединения с базой данных
type DatabaseOptions struct {
	// Имена таблиц в единственном числе
	SingularTable bool
	// Уровень логирования GORM
	LogLevel goormlogger.LogLevel
	// Максимальное количество простаивающих соединений
	MaxIdleConns int
	// Максимальное количество открытых соединений
	MaxOpenConns int
	// Максимальное время жизни соединения
	ConnMaxLifetime time.Duration
}

// DefaultDatabaseOptions возвращает опции по умолчанию
func DefaultDatabaseOptions() *DatabaseOptions {
	return &DatabaseOptions{
		SingularTable:   true,
		LogLevel:        goormlogger.Info,
		MaxIdleConns:    10,
		MaxOpenConns:    100,
		ConnMaxLifetime: time.Hour,
	}
}

// NewDatabase создает новое соединение с базой данных
func NewDatabase(databaseURL string, logger logging.Logger, options *DatabaseOptions) (*Database, error) {
	if logger == nil {
		logger = logging.NewLogger()
	}

	if options == nil {
		options = DefaultDatabaseOptions()
	}

	// Настраиваем конфигурацию GORM
	config := &gorm.Config{
		Logger: goormlogger.Default.LogMode(options.LogLevel),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: options.SingularTable,
		},
	}

	// Подключаемся к базе данных
	db, err := gorm.Open(postgres.Open(databaseURL), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Настраиваем пул соединений
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	// Устанавливаем параметры пула соединений
	sqlDB.SetMaxIdleConns(options.MaxIdleConns)
	sqlDB.SetMaxOpenConns(options.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(options.ConnMaxLifetime)

	logger.Info("Successfully connected to database")

	return &Database{
		db:     db,
		logger: logger,
	}, nil
}

// GetDB возвращает экземпляр GORM DB
func (db *Database) GetDB() *gorm.DB {
	return db.db
}

// Close закрывает соединение с базой данных
func (d *Database) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %v", err)
	}

	d.logger.Info("Database connection closed")
	return nil
}

// Ping проверяет соединение с базой данных
func (d *Database) Ping() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %v", err)
	}

	return nil
}

// AutoMigrate выполняет автоматическую миграцию моделей
func (d *Database) AutoMigrate(models ...interface{}) error {
	if err := d.db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migration failed: %v", err)
	}

	return nil
}

// WithLogger возвращает новый экземпляр Database с указанным логгером
func (d *Database) WithLogger(logger logging.Logger) *Database {
	return &Database{
		db:     d.db.Session(&gorm.Session{}),
		logger: logger,
	}
}

// Transaction выполняет функцию в транзакции
func (d *Database) Transaction(txFunc func(tx *gorm.DB) error) error {
	return d.db.Transaction(txFunc)
}
