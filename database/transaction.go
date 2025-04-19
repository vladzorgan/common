package database

import (
	"context"

	"github.com/vladzorgan/common/logging"
	"gorm.io/gorm"
)

// TransactionKey - ключ для хранения транзакции в контексте
type TransactionKey struct{}

// TxProvider предоставляет интерфейс для получения транзакции
type TxProvider interface {
	GetTx(ctx context.Context) *gorm.DB
}

// GormTxProvider реализация TxProvider для GORM
type GormTxProvider struct {
	db *Database
}

// NewGormTxProvider создает новый провайдер транзакций
func NewGormTxProvider(db *Database) *GormTxProvider {
	return &GormTxProvider{db: db}
}

// GetTx возвращает транзакцию из контекста или создает новую сессию, если транзакция не найдена
func (p *GormTxProvider) GetTx(ctx context.Context) *gorm.DB {
	tx, ok := ctx.Value(TransactionKey{}).(*gorm.DB)
	if ok && tx != nil {
		return tx
	}
	return p.db.GetDB().WithContext(ctx)
}

// RunInTransaction выполняет функцию в транзакции
func RunInTransaction(ctx context.Context, db *Database, fn func(ctx context.Context) error) error {
	return db.GetDB().Transaction(func(tx *gorm.DB) error {
		// Создаем новый контекст с транзакцией
		txCtx := context.WithValue(ctx, TransactionKey{}, tx)
		return fn(txCtx)
	})
}

// WithTransaction создает новый контекст с транзакцией
func WithTransaction(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, TransactionKey{}, tx)
}

// TransactionMiddleware middleware для автоматического управления транзакциями в HTTP запросах
type TransactionMiddleware struct {
	db     *Database
	logger logging.Logger
}

// NewTransactionMiddleware создает новый middleware для управления транзакциями
func NewTransactionMiddleware(db *Database, logger logging.Logger) *TransactionMiddleware {
	return &TransactionMiddleware{
		db:     db,
		logger: logger,
	}
}

// Handler обрабатывает HTTP запрос с транзакцией
func (m *TransactionMiddleware) Handler(next func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Проверяем, есть ли уже транзакция в контексте
		if _, ok := ctx.Value(TransactionKey{}).(*gorm.DB); ok {
			// Если транзакция уже есть, просто выполняем обработчик
			return next(ctx)
		}

		// Начинаем новую транзакцию
		var err error
		err = m.db.GetDB().Transaction(func(tx *gorm.DB) error {
			// Создаем новый контекст с транзакцией
			txCtx := context.WithValue(ctx, TransactionKey{}, tx)

			// Выполняем обработчик
			if err := next(txCtx); err != nil {
				// Возвращаем ошибку для отката транзакции
				return err
			}

			// Транзакция будет зафиксирована при возврате nil
			return nil
		})

		return err
	}
}

// Repository представляет базовый репозиторий с поддержкой транзакций
type Repository struct {
	db         *Database
	logger     logging.Logger
	txProvider TxProvider
}

// NewRepository создает новый базовый репозиторий
func NewRepository(db *Database, logger logging.Logger) *Repository {
	return &Repository{
		db:         db,
		logger:     logger,
		txProvider: NewGormTxProvider(db),
	}
}

// WithTxProvider устанавливает провайдер транзакций
func (r *Repository) WithTxProvider(txProvider TxProvider) *Repository {
	r.txProvider = txProvider
	return r
}

// DB возвращает транзакцию из контекста или создает новую сессию
func (r *Repository) DB(ctx context.Context) *gorm.DB {
	return r.txProvider.GetTx(ctx)
}

// Transaction выполняет функцию в транзакции
func (r *Repository) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	// Проверяем, есть ли уже транзакция в контексте
	if _, ok := ctx.Value(TransactionKey{}).(*gorm.DB); ok {
		// Если транзакция уже есть, просто выполняем функцию
		return fn(ctx)
	}

	// Начинаем новую транзакцию
	return RunInTransaction(ctx, r.db, fn)
}
