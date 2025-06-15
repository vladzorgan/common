package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vladzorgan/common/repository"
	events "github.com/vladzorgan/common/messaging/rabbitmq"
)

// BaseEntity представляет базовую сущность с общими полями
type BaseEntity interface {
	repository.BaseModel
	GetName() string
}

// PaginationResponse представляет структуру ответа с пагинацией
type PaginationResponse[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// Pagination представляет информацию о пагинации
type Pagination struct {
	Total int `json:"total"`
	Page  int `json:"page"`
	Size  int `json:"size"`
	Pages int `json:"pages"`
}

// Service определяет универсальный интерфейс сервиса
type Service[T BaseEntity, R any] interface {
	// CRUD операции
	Create(ctx context.Context, input CreateInput[T]) (*R, error)
	GetByID(ctx context.Context, id uint) (*R, error)
	Update(ctx context.Context, id uint, input UpdateInput[T]) (*R, error)
	Delete(ctx context.Context, id uint) (*R, error)
	
	// Массовые операции
	BulkCreate(ctx context.Context, inputs []CreateInput[T]) ([]R, error)
	BulkUpdate(ctx context.Context, updates []BulkUpdateInput[T]) ([]R, error)
	
	// Операции с коллекциями
	GetAll(ctx context.Context, skip, limit int, filters map[string]interface{}, sort *repository.SortOptions) (*PaginationResponse[R], error)
	Search(ctx context.Context, keyword string, skip, limit int, filters map[string]interface{}, sort *repository.SortOptions) (*PaginationResponse[R], error)
	GetByField(ctx context.Context, field string, value interface{}) (*R, error)
	GetAllByField(ctx context.Context, field string, value interface{}, skip, limit int) (*PaginationResponse[R], error)
	
	// Дополнительные операции
	Count(ctx context.Context, filters map[string]interface{}) (int64, error)
	Exists(ctx context.Context, id uint) (bool, error)
}

// CreateInput представляет входные данные для создания
type CreateInput[T BaseEntity] interface {
	ToEntity() *T
	Validate() error
}

// UpdateInput представляет входные данные для обновления
type UpdateInput[T BaseEntity] interface {
	ToUpdateMap() map[string]interface{}
	Validate() error
}

// BulkUpdateInput представляет входные данные для массового обновления
type BulkUpdateInput[T BaseEntity] interface {
	GetID() uint
	ToUpdateMap() map[string]interface{}
	Validate() error
}

// EntityTransformer определяет интерфейс для преобразования сущностей
type EntityTransformer[T BaseEntity, R any] interface {
	Transform(entity *T) *R
	TransformSlice(entities []T) []R
}

// BaseService представляет базовую реализацию сервиса
type BaseService[T BaseEntity, R any] struct {
	repo        repository.Repository[T]
	transformer EntityTransformer[T, R]
	publisher   *events.Publisher
	entityName  string
}

// NewBaseService создает новый экземпляр BaseService
func NewBaseService[T BaseEntity, R any](
	repo repository.Repository[T],
	transformer EntityTransformer[T, R],
	publisher *events.Publisher,
	entityName string,
) *BaseService[T, R] {
	return &BaseService[T, R]{
		repo:        repo,
		transformer: transformer,
		publisher:   publisher,
		entityName:  entityName,
	}
}

// Create создает новую сущность
func (s *BaseService[T, R]) Create(ctx context.Context, input CreateInput[T]) (*R, error) {
	// Валидация входных данных
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("ошибка валидации: %v", err)
	}
	
	// Создаем сущность
	entity := input.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return nil, fmt.Errorf("не удалось создать %s: %v", s.entityName, err)
	}
	
	log.Printf("Создан новый %s: %s (ID: %d)", s.entityName, (*entity).GetName(), (*entity).GetID())
	
	// Публикуем событие о создании
	if s.publisher != nil {
		s.publishEvent(ctx, "created", entity, nil)
	}
	
	// Преобразуем в ответ
	response := s.transformer.Transform(entity)
	return response, nil
}

// BulkCreate создает множество новых сущностей
func (s *BaseService[T, R]) BulkCreate(ctx context.Context, inputs []CreateInput[T]) ([]R, error) {
	if len(inputs) == 0 {
		return []R{}, nil
	}
	
	// Валидация всех входных данных
	entities := make([]*T, 0, len(inputs))
	for i, input := range inputs {
		if err := input.Validate(); err != nil {
			return nil, fmt.Errorf("ошибка валидации элемента %d: %v", i, err)
		}
		entities = append(entities, input.ToEntity())
	}
	
	// Массовое создание в репозитории
	if err := s.repo.BulkCreate(ctx, entities); err != nil {
		return nil, fmt.Errorf("не удалось создать %s: %v", s.entityName, err)
	}
	
	log.Printf("Создано %d новых %s", len(entities), s.entityName)
	
	// Публикуем событие о массовом создании
	if s.publisher != nil {
		s.publishBulkEvent(ctx, "bulk_created", entities)
	}
	
	// Преобразуем сущности в ответы
	responses := make([]R, 0, len(entities))
	for _, entity := range entities {
		response := s.transformer.Transform(entity)
		responses = append(responses, *response)
	}
	
	return responses, nil
}

// BulkUpdate обновляет множество сущностей
func (s *BaseService[T, R]) BulkUpdate(ctx context.Context, inputs []BulkUpdateInput[T]) ([]R, error) {
	if len(inputs) == 0 {
		return []R{}, nil
	}
	
	// Валидация всех входных данных и подготовка данных для обновления
	updates := make([]repository.BulkUpdateItem, 0, len(inputs))
	updatedIDs := make([]uint, 0, len(inputs))
	
	for i, input := range inputs {
		if err := input.Validate(); err != nil {
			return nil, fmt.Errorf("ошибка валидации элемента %d: %v", i, err)
		}
		
		updateMap := input.ToUpdateMap()
		if len(updateMap) == 0 {
			continue // Пропускаем элементы без изменений
		}
		
		updates = append(updates, repository.BulkUpdateItem{
			ID:      input.GetID(),
			Updates: updateMap,
		})
		updatedIDs = append(updatedIDs, input.GetID())
	}
	
	if len(updates) == 0 {
		return []R{}, nil
	}
	
	// Массовое обновление в репозитории
	if err := s.repo.BulkUpdate(ctx, updates); err != nil {
		return nil, fmt.Errorf("не удалось обновить %s: %v", s.entityName, err)
	}
	
	log.Printf("Обновлено %d %s", len(updates), s.entityName)
	
	// Получаем обновленные сущности для возврата
	responses := make([]R, 0, len(updatedIDs))
	for _, id := range updatedIDs {
		entity, err := s.repo.GetByID(ctx, id)
		if err != nil {
			log.Printf("Ошибка при получении обновленной сущности %s с ID %d: %v", s.entityName, id, err)
			continue
		}
		if entity != nil {
			response := s.transformer.Transform(entity)
			responses = append(responses, *response)
		}
	}
	
	// Публикуем событие о массовом обновлении
	if s.publisher != nil {
		entities := make([]*T, 0, len(responses))
		for _, id := range updatedIDs {
			if entity, err := s.repo.GetByID(ctx, id); err == nil && entity != nil {
				entities = append(entities, entity)
			}
		}
		s.publishBulkEvent(ctx, "bulk_updated", entities)
	}
	
	return responses, nil
}

// GetByID получает сущность по ID
func (s *BaseService[T, R]) GetByID(ctx context.Context, id uint) (*R, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении %s: %v", s.entityName, err)
	}
	
	if entity == nil {
		return nil, fmt.Errorf("%s с ID %d не найден", s.entityName, id)
	}
	
	response := s.transformer.Transform(entity)
	return response, nil
}

// Update обновляет сущность
func (s *BaseService[T, R]) Update(ctx context.Context, id uint, input UpdateInput[T]) (*R, error) {
	// Проверяем существование сущности
	exists, err := s.repo.Exists(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка при проверке существования %s: %v", s.entityName, err)
	}
	
	if !exists {
		return nil, fmt.Errorf("%s с ID %d не найден", s.entityName, id)
	}
	
	// Валидация входных данных
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("ошибка валидации: %v", err)
	}
	
	// Получаем данные для обновления
	updates := input.ToUpdateMap()
	if len(updates) == 0 {
		return nil, fmt.Errorf("нет данных для обновления")
	}
	
	// Обновляем сущность
	updatedEntity, err := s.repo.Update(ctx, id, updates)
	if err != nil {
		return nil, fmt.Errorf("не удалось обновить %s: %v", s.entityName, err)
	}
	
	if updatedEntity == nil {
		return nil, fmt.Errorf("%s с ID %d не найден", s.entityName, id)
	}
	
	log.Printf("Обновлен %s: %s (ID: %d)", s.entityName, (*updatedEntity).GetName(), (*updatedEntity).GetID())
	
	// Публикуем событие об обновлении
	if s.publisher != nil {
		updatedFields := make([]string, 0, len(updates))
		for key := range updates {
			updatedFields = append(updatedFields, key)
		}
		s.publishEvent(ctx, "updated", updatedEntity, updatedFields)
	}
	
	response := s.transformer.Transform(updatedEntity)
	return response, nil
}

// Delete удаляет сущность
func (s *BaseService[T, R]) Delete(ctx context.Context, id uint) (*R, error) {
	// Получаем сущность перед удалением
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении %s: %v", s.entityName, err)
	}
	
	if entity == nil {
		return nil, fmt.Errorf("%s с ID %d не найден", s.entityName, id)
	}
	
	// Сохраняем данные для ответа
	response := s.transformer.Transform(entity)
	
	// Удаляем сущность
	deletedEntity, err := s.repo.Delete(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("не удалось удалить %s: %v", s.entityName, err)
	}
	
	if deletedEntity == nil {
		return nil, fmt.Errorf("%s с ID %d не найден", s.entityName, id)
	}
	
	log.Printf("Удален %s: %s (ID: %d)", s.entityName, (*deletedEntity).GetName(), (*deletedEntity).GetID())
	
	// Публикуем событие об удалении
	if s.publisher != nil {
		s.publishEvent(ctx, "deleted", deletedEntity, nil)
	}
	
	return response, nil
}

// GetAll получает все сущности с пагинацией, фильтрацией и сортировкой
func (s *BaseService[T, R]) GetAll(ctx context.Context, skip, limit int, filters map[string]interface{}, sort *repository.SortOptions) (*PaginationResponse[R], error) {
	entities, total, err := s.repo.GetAll(ctx, skip, limit, filters, sort)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении списка %s: %v", s.entityName, err)
	}
	
	// Преобразуем сущности в ответы
	responses := s.transformer.TransformSlice(entities)
	
	// Вычисляем пагинацию
	pagination := s.calculatePagination(total, skip, limit)
	
	return &PaginationResponse[R]{
		Items:      responses,
		Pagination: pagination,
	}, nil
}

// Search выполняет поиск сущностей с сортировкой
func (s *BaseService[T, R]) Search(ctx context.Context, keyword string, skip, limit int, filters map[string]interface{}, sort *repository.SortOptions) (*PaginationResponse[R], error) {
	// Запуск таймера для измерения производительности
	startTime := time.Now()
	
	entities, total, err := s.repo.Search(ctx, keyword, skip, limit, filters, sort)
	if err != nil {
		return nil, fmt.Errorf("ошибка при поиске %s: %v", s.entityName, err)
	}
	
	// Логируем поисковый запрос
	processingTime := int(time.Since(startTime).Milliseconds())
	
	log.Printf("Поиск %s по запросу '%s': найдено %d результатов за %d мс", 
		s.entityName, keyword, len(entities), processingTime)
	
	// Преобразуем сущности в ответы
	responses := s.transformer.TransformSlice(entities)
	
	// Вычисляем пагинацию
	pagination := s.calculatePagination(total, skip, limit)
	
	return &PaginationResponse[R]{
		Items:      responses,
		Pagination: pagination,
	}, nil
}

// Count подсчитывает количество сущностей
func (s *BaseService[T, R]) Count(ctx context.Context, filters map[string]interface{}) (int64, error) {
	count, err := s.repo.Count(ctx, filters)
	if err != nil {
		return 0, fmt.Errorf("ошибка при подсчете %s: %v", s.entityName, err)
	}
	
	return count, nil
}

// Exists проверяет существование сущности
func (s *BaseService[T, R]) Exists(ctx context.Context, id uint) (bool, error) {
	exists, err := s.repo.Exists(ctx, id)
	if err != nil {
		return false, fmt.Errorf("ошибка при проверке существования %s: %v", s.entityName, err)
	}
	
	return exists, nil
}

// GetByField получает сущность по указанному полю
func (s *BaseService[T, R]) GetByField(ctx context.Context, field string, value interface{}) (*R, error) {
	entity, err := s.repo.GetByField(ctx, field, value)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении %s по полю %s: %v", s.entityName, field, err)
	}
	
	if entity == nil {
		return nil, fmt.Errorf("%s с %s = %v не найден", s.entityName, field, value)
	}
	
	response := s.transformer.Transform(entity)
	return response, nil
}

// GetAllByField получает все сущности по указанному полю с пагинацией
func (s *BaseService[T, R]) GetAllByField(ctx context.Context, field string, value interface{}, skip, limit int) (*PaginationResponse[R], error) {
	entities, total, err := s.repo.GetAllByField(ctx, field, value, skip, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении списка %s по полю %s: %v", s.entityName, field, err)
	}
	
	// Преобразуем сущности в ответы
	responses := s.transformer.TransformSlice(entities)
	
	// Вычисляем пагинацию
	pagination := s.calculatePagination(total, skip, limit)
	
	return &PaginationResponse[R]{
		Items:      responses,
		Pagination: pagination,
	}, nil
}

// calculatePagination вычисляет информацию о пагинации
func (s *BaseService[T, R]) calculatePagination(total int64, skip, limit int) Pagination {
	// Вычисляем количество страниц
	pages := (int(total) + limit - 1) / limit
	if limit <= 0 {
		pages = 0
	}
	
	// Номер текущей страницы
	page := (skip / limit) + 1
	if limit <= 0 {
		page = 1
	}
	
	return Pagination{
		Total: int(total),
		Page:  page,
		Size:  limit,
		Pages: pages,
	}
}

// publishEvent публикует событие в очередь сообщений
func (s *BaseService[T, R]) publishEvent(ctx context.Context, eventType string, entity *T, updatedFields []string) {
	eventData := map[string]interface{}{
		"id":          (*entity).GetID(),
		"name":        (*entity).GetName(),
		"event_type":  eventType,
		"entity_type": s.entityName,
	}
	
	if updatedFields != nil {
		eventData["updated_fields"] = updatedFields
	}
	
	eventName := fmt.Sprintf("%s.%s", s.entityName, eventType)
	if err := s.publisher.PublishEvent(ctx, eventName, eventData); err != nil {
		log.Printf("Ошибка при публикации события %s: %v", eventName, err)
	}
}

// publishBulkEvent публикует событие массовой операции в очередь сообщений
func (s *BaseService[T, R]) publishBulkEvent(ctx context.Context, eventType string, entities []*T) {
	if len(entities) == 0 {
		return
	}
	
	entityIDs := make([]uint, 0, len(entities))
	entityNames := make([]string, 0, len(entities))
	
	for _, entity := range entities {
		entityIDs = append(entityIDs, (*entity).GetID())
		entityNames = append(entityNames, (*entity).GetName())
	}
	
	eventData := map[string]interface{}{
		"ids":         entityIDs,
		"names":       entityNames,
		"count":       len(entities),
		"event_type":  eventType,
		"entity_type": s.entityName,
	}
	
	eventName := fmt.Sprintf("%s.%s", s.entityName, eventType)
	if err := s.publisher.PublishEvent(ctx, eventName, eventData); err != nil {
		log.Printf("Ошибка при публикации массового события %s: %v", eventName, err)
	}
}