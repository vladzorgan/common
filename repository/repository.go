package repository

import (
	"context"
	"github.com/vladzorgan/common/auth"
	"github.com/vladzorgan/common/database"
	"gorm.io/gorm"
)

// BaseModel представляет базовую модель с общими полями
type BaseModel interface {
	GetID() uint
	GetTableName() string
}

// OwnableModel представляет модель с поддержкой владения
type OwnableModel interface {
	BaseModel
	GetOwnerID() uint // Возвращает ID владельца сущности
}

// AuthConfig определяет настройки авторизации для репозитория
type AuthConfig struct {
	ResourceType auth.ResourceType // Тип ресурса
	OwnerField   string            // Поле владельца в базе данных (например, "user_id")
	Enabled      bool              // Включена ли авторизация
	ReadAuth     bool              // Требуется ли авторизация для чтения
	WriteAuth    bool              // Требуется ли авторизация для записи
}

// SortOptions определяет параметры сортировки
type SortOptions struct {
	Field string // Поле для сортировки
	Order string // Порядок сортировки: "asc" или "desc"
}

// Repository определяет универсальный интерфейс репозитория
type Repository[T BaseModel] interface {
	// CRUD операции
	Create(ctx context.Context, entity *T) error
	GetByID(ctx context.Context, id uint) (*T, error)
	Update(ctx context.Context, id uint, updates map[string]interface{}) (*T, error)
	Delete(ctx context.Context, id uint) (*T, error)
	
	// Операции с коллекциями
	GetAll(ctx context.Context, skip, limit int, filters map[string]interface{}, sort *SortOptions) ([]T, int64, error)
	Search(ctx context.Context, keyword string, skip, limit int, filters map[string]interface{}, sort *SortOptions) ([]T, int64, error)
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
	db         *database.Database
	tx         *gorm.DB
	authConfig *AuthConfig
}

// NewBaseRepository создает новый экземпляр BaseRepository
func NewBaseRepository[T BaseModel](db *database.Database) *BaseRepository[T] {
	return &BaseRepository[T]{
		db: db,
	}
}

// NewBaseRepositoryWithAuth создает новый экземпляр BaseRepository с авторизацией
func NewBaseRepositoryWithAuth[T BaseModel](db *database.Database, authConfig *AuthConfig) *BaseRepository[T] {
	return &BaseRepository[T]{
		db:         db,
		authConfig: authConfig,
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
		db:         r.db,
		tx:         tx,
		authConfig: r.authConfig,
	}
}

// Create создает новую запись в базе данных
func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
	// Проверяем разрешения на запись
	if err := r.checkWritePermission(ctx); err != nil {
		return err
	}

	if err := r.getDB().WithContext(ctx).Create(entity).Error; err != nil {
		return err
	}
	return nil
}

// GetByID получает запись по ID
func (r *BaseRepository[T]) GetByID(ctx context.Context, id uint) (*T, error) {
	// Проверяем разрешения на чтение
	if err := r.checkReadPermission(ctx); err != nil {
		return nil, err
	}

	var entity T
	
	query := r.getDB().WithContext(ctx)
	// Применяем фильтр по владению если настроен
	query = r.applyOwnershipFilter(ctx, query)
	
	if err := query.First(&entity, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	
	// Дополнительная проверка владения для конкретной записи
	if err := r.checkOwnership(ctx, &entity); err != nil {
		return nil, err
	}
	
	return &entity, nil
}

// Update обновляет запись по ID
func (r *BaseRepository[T]) Update(ctx context.Context, id uint, updates map[string]interface{}) (*T, error) {
	// Проверяем разрешения на запись
	if err := r.checkWritePermission(ctx); err != nil {
		return nil, err
	}

	var entity T
	
	query := r.getDB().WithContext(ctx)
	// Применяем фильтр по владению
	query = r.applyOwnershipFilter(ctx, query)
	
	// Получаем запись для обновления
	if err := query.First(&entity, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	
	// Проверяем права владения
	if err := r.checkOwnership(ctx, &entity); err != nil {
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
	// Проверяем разрешения на запись (для удаления)
	if err := r.checkWritePermission(ctx); err != nil {
		return nil, err
	}

	var entity T
	
	query := r.getDB().WithContext(ctx)
	// Применяем фильтр по владению
	query = r.applyOwnershipFilter(ctx, query)
	
	// Получаем запись перед удалением
	if err := query.First(&entity, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	
	// Проверяем права владения
	if err := r.checkOwnership(ctx, &entity); err != nil {
		return nil, err
	}
	
	// Удаляем запись
	if err := r.getDB().WithContext(ctx).Delete(&entity).Error; err != nil {
		return nil, err
	}
	
	return &entity, nil
}

// GetAll получает все записи с пагинацией, фильтрацией и сортировкой
func (r *BaseRepository[T]) GetAll(ctx context.Context, skip, limit int, filters map[string]interface{}, sort *SortOptions) ([]T, int64, error) {
	var entities []T
	var total int64
	
	// Создаем базовый запрос
	query := r.getDB().WithContext(ctx).Model(new(T))
	queryCount := r.getDB().WithContext(ctx).Model(new(T))
	
	// Проверяем разрешения на чтение
	if err := r.checkReadPermission(ctx); err != nil {
		return nil, 0, err
	}

	// Применяем фильтр по владению
	query = r.applyOwnershipFilter(ctx, query)
	queryCount = r.applyOwnershipFilter(ctx, queryCount)

	// Применяем фильтры
	query = r.applyFilters(query, filters)
	queryCount = r.applyFilters(queryCount, filters)
	
	// Применяем сортировку
	query = r.applySorting(query, sort)
	
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

// Search выполняет поиск записей по ключевому слову с сортировкой
func (r *BaseRepository[T]) Search(ctx context.Context, keyword string, skip, limit int, filters map[string]interface{}, sort *SortOptions) ([]T, int64, error) {
	var entities []T
	var total int64
	
	searchQuery := "%" + keyword + "%"
	
	// Создаем базовый запрос с поиском
	query := r.getDB().WithContext(ctx).Model(new(T)).
		Where("name ILIKE ?", searchQuery)
	queryCount := r.getDB().WithContext(ctx).Model(new(T)).
		Where("name ILIKE ?", searchQuery)
	
	// Проверяем разрешения на чтение
	if err := r.checkReadPermission(ctx); err != nil {
		return nil, 0, err
	}

	// Применяем фильтр по владению
	query = r.applyOwnershipFilter(ctx, query)
	queryCount = r.applyOwnershipFilter(ctx, queryCount)

	// Применяем дополнительные фильтры
	query = r.applyFilters(query, filters)
	queryCount = r.applyFilters(queryCount, filters)
	
	// Применяем сортировку
	query = r.applySorting(query, sort)
	
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

// applySorting применяет сортировку к запросу
func (r *BaseRepository[T]) applySorting(query *gorm.DB, sort *SortOptions) *gorm.DB {
	if sort == nil || sort.Field == "" {
		// Сортировка по умолчанию - по ID в порядке возрастания
		return query.Order("id ASC")
	}
	
	// Определяем допустимые поля для сортировки
	allowedFields := map[string]bool{
		"id":         true,
		"name":       true,
		"created_at": true,
		"updated_at": true,
	}
	
	// Проверяем, что поле разрешено для сортировки
	if !allowedFields[sort.Field] {
		// Если поле не разрешено, используем сортировку по умолчанию
		return query.Order("id ASC")
	}
	
	// Определяем порядок сортировки
	order := "ASC"
	if sort.Order == "desc" || sort.Order == "DESC" {
		order = "DESC"
	}
	
	return query.Order(sort.Field + " " + order)
}

// checkReadPermission проверяет разрешения на чтение
func (r *BaseRepository[T]) checkReadPermission(ctx context.Context) error {
	if r.authConfig == nil || !r.authConfig.Enabled || !r.authConfig.ReadAuth {
		return nil
	}

	check := auth.PermissionCheck{
		Resource:   r.authConfig.ResourceType,
		Permission: auth.PermissionRead,
	}

	_, err := auth.RequirePermission(ctx, check)
	return err
}

// checkWritePermission проверяет разрешения на запись
func (r *BaseRepository[T]) checkWritePermission(ctx context.Context) error {
	if r.authConfig == nil || !r.authConfig.Enabled || !r.authConfig.WriteAuth {
		return nil
	}

	check := auth.PermissionCheck{
		Resource:   r.authConfig.ResourceType,
		Permission: auth.PermissionWrite,
	}

	_, err := auth.RequirePermission(ctx, check)
	return err
}

// checkOwnership проверяет права владения для конкретной сущности
func (r *BaseRepository[T]) checkOwnership(ctx context.Context, entity *T) error {
	if r.authConfig == nil || !r.authConfig.Enabled || r.authConfig.OwnerField == "" {
		return nil
	}

	// Проверяем, реализует ли сущность интерфейс OwnableModel
	if ownableEntity, ok := any(*entity).(OwnableModel); ok {
		ownerID := ownableEntity.GetOwnerID()
		return auth.CheckOwnership(ctx, ownerID)
	}

	return nil
}

// applyOwnershipFilter применяет фильтр по владению для обычных пользователей
func (r *BaseRepository[T]) applyOwnershipFilter(ctx context.Context, query *gorm.DB) *gorm.DB {
	if r.authConfig == nil || !r.authConfig.Enabled || r.authConfig.OwnerField == "" {
		return query
	}

	// Получаем пользователя из контекста
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		// Если пользователь не найден, возвращаем пустой результат
		return query.Where("1 = 0")
	}

	// Администраторы видят все записи
	if user.IsAdmin() {
		return query
	}

	// Для обычных пользователей применяем фильтр по владению
	return query.Where(r.authConfig.OwnerField+" = ?", user.ID)
}