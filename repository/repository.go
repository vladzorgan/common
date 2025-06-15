package repository

import (
	"context"
	"github.com/vladzorgan/common/database"
	"gorm.io/gorm"
)

// BaseModel представляет базовую модель с общими полями
type BaseModel interface {
	GetID() uint
	GetTableName() string
}

// Repository определяет универсальный интерфейс репозитория
type Repository[T BaseModel] interface {
	// CRUD операции
	Create(ctx context.Context, entity *T) error
	GetByID(ctx context.Context, id uint) (*T, error)
	Update(ctx context.Context, id uint, updates map[string]interface{}) (*T, error)
	Delete(ctx context.Context, id uint) (*T, error)
	
	// Операции с коллекциями
	GetAll(ctx context.Context, skip, limit int, filters map[string]interface{}) ([]T, int64, error)
	Search(ctx context.Context, keyword string, skip, limit int, filters map[string]interface{}) ([]T, int64, error)
	GetByField(ctx context.Context, field string, value interface{}) (*T, error)
	GetAllByField(ctx context.Context, field string, value interface{}, skip, limit int) ([]T, int64, error)
	
	// Дополнительные операции
	Count(ctx context.Context, filters map[string]interface{}) (int64, error)
	Exists(ctx context.Context, id uint) (bool, error)
	
	// Работа с транзакциями
	WithTx(tx *gorm.DB) Repository[T]
}

// BaseRepository представляет базовую реализацию репозитория
type BaseRepository[T BaseModel] struct {
	db *database.Database
	tx *gorm.DB
}

// NewBaseRepository создает новый экземпляр BaseRepository
func NewBaseRepository[T BaseModel](db *database.Database) *BaseRepository[T] {
	return &BaseRepository[T]{
		db: db,
	}
}

// getDB возвращает подключение к базе данных (обычное или транзакция)
func (r *BaseRepository[T]) getDB() *gorm.DB {
	if r.tx != nil {
		return r.tx
	}
	return r.db.GetDB()
}

// WithTx создает новый репозиторий с транзакцией
func (r *BaseRepository[T]) WithTx(tx *gorm.DB) Repository[T] {
	return &BaseRepository[T]{
		db: r.db,
		tx: tx,
	}
}

// Create создает новую запись в базе данных
func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
	if err := r.getDB().WithContext(ctx).Create(entity).Error; err != nil {
		return err
	}
	return nil
}

// GetByID получает запись по ID
func (r *BaseRepository[T]) GetByID(ctx context.Context, id uint) (*T, error) {
	var entity T
	
	if err := r.getDB().WithContext(ctx).First(&entity, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	
	return &entity, nil
}

// Update обновляет запись по ID
func (r *BaseRepository[T]) Update(ctx context.Context, id uint, updates map[string]interface{}) (*T, error) {
	var entity T
	
	// Получаем запись для обновления
	if err := r.getDB().WithContext(ctx).First(&entity, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	
	// Обновляем запись
	if err := r.getDB().WithContext(ctx).Model(&entity).Updates(updates).Error; err != nil {
		return nil, err
	}
	
	// Получаем обновленную запись
	if err := r.getDB().WithContext(ctx).First(&entity, id).Error; err != nil {
		return nil, err
	}
	
	return &entity, nil
}

// Delete удаляет запись по ID (soft delete)
func (r *BaseRepository[T]) Delete(ctx context.Context, id uint) (*T, error) {
	var entity T
	
	// Получаем запись перед удалением
	if err := r.getDB().WithContext(ctx).First(&entity, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	
	// Удаляем запись
	if err := r.getDB().WithContext(ctx).Delete(&entity).Error; err != nil {
		return nil, err
	}
	
	return &entity, nil
}

// GetAll получает все записи с пагинацией и фильтрацией
func (r *BaseRepository[T]) GetAll(ctx context.Context, skip, limit int, filters map[string]interface{}) ([]T, int64, error) {
	var entities []T
	var total int64
	
	// Создаем базовый запрос
	query := r.getDB().WithContext(ctx).Model(new(T))
	queryCount := r.getDB().WithContext(ctx).Model(new(T))
	
	// Применяем фильтры
	query = r.applyFilters(query, filters)
	queryCount = r.applyFilters(queryCount, filters)
	
	// Получаем общее количество записей
	if err := queryCount.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// Получаем записи с пагинацией
	if err := query.
		Limit(limit).
		Offset(skip).
		Find(&entities).Error; err != nil {
		return nil, 0, err
	}
	
	return entities, total, nil
}

// Search выполняет поиск записей по ключевому слову
func (r *BaseRepository[T]) Search(ctx context.Context, keyword string, skip, limit int, filters map[string]interface{}) ([]T, int64, error) {
	var entities []T
	var total int64
	
	searchQuery := "%" + keyword + "%"
	
	// Создаем базовый запрос с поиском
	query := r.getDB().WithContext(ctx).Model(new(T)).
		Where("name ILIKE ?", searchQuery)
	queryCount := r.getDB().WithContext(ctx).Model(new(T)).
		Where("name ILIKE ?", searchQuery)
	
	// Применяем дополнительные фильтры
	query = r.applyFilters(query, filters)
	queryCount = r.applyFilters(queryCount, filters)
	
	// Получаем общее количество найденных записей
	if err := queryCount.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// Получаем записи с пагинацией
	if err := query.
		Limit(limit).
		Offset(skip).
		Find(&entities).Error; err != nil {
		return nil, 0, err
	}
	
	return entities, total, nil
}

// Count подсчитывает количество записей с фильтрами
func (r *BaseRepository[T]) Count(ctx context.Context, filters map[string]interface{}) (int64, error) {
	var count int64
	
	query := r.getDB().WithContext(ctx).Model(new(T))
	query = r.applyFilters(query, filters)
	
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	
	return count, nil
}

// Exists проверяет существование записи по ID
func (r *BaseRepository[T]) Exists(ctx context.Context, id uint) (bool, error) {
	var count int64
	
	if err := r.getDB().WithContext(ctx).
		Model(new(T)).
		Where("id = ?", id).
		Count(&count).Error; err != nil {
		return false, err
	}
	
	return count > 0, nil
}

// GetByField получает запись по указанному полю
func (r *BaseRepository[T]) GetByField(ctx context.Context, field string, value interface{}) (*T, error) {
	var entity T
	
	if err := r.getDB().WithContext(ctx).Where(field+" = ?", value).First(&entity).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	
	return &entity, nil
}

// GetAllByField получает все записи по указанному полю с пагинацией
func (r *BaseRepository[T]) GetAllByField(ctx context.Context, field string, value interface{}, skip, limit int) ([]T, int64, error) {
	var entities []T
	var total int64
	
	// Создаем базовый запрос
	query := r.getDB().WithContext(ctx).Model(new(T)).Where(field+" = ?", value)
	queryCount := r.getDB().WithContext(ctx).Model(new(T)).Where(field+" = ?", value)
	
	// Получаем общее количество записей
	if err := queryCount.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// Получаем записи с пагинацией
	if err := query.
		Limit(limit).
		Offset(skip).
		Find(&entities).Error; err != nil {
		return nil, 0, err
	}
	
	return entities, total, nil
}

// applyFilters применяет фильтры к запросу
func (r *BaseRepository[T]) applyFilters(query *gorm.DB, filters map[string]interface{}) *gorm.DB {
	for key, value := range filters {
		if value != nil && value != "" {
			switch key {
			case "id":
				query = query.Where("id = ?", value)
			case "ids":
				query = query.Where("id IN ?", value)
			case "name":
				if str, ok := value.(string); ok {
					query = query.Where("name ILIKE ?", "%"+str+"%")
				}
			case "created_after":
				query = query.Where("created_at > ?", value)
			case "created_before":
				query = query.Where("created_at < ?", value)
			case "updated_after":
				query = query.Where("updated_at > ?", value)
			case "updated_before":
				query = query.Where("updated_at < ?", value)
			default:
				// Для всех остальных полей применяем точное совпадение
				query = query.Where(key+" = ?", value)
			}
		}
	}
	return query
}