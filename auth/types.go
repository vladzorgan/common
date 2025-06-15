package auth

import (
	"time"
)

// UserRole определяет роли пользователей в системе
type UserRole string

const (
	UserRole_User            UserRole = "user"             // Обычный пользователь
	UserRole_ServiceOwner    UserRole = "service_owner"    // Владелец сервисного центра
	UserRole_ServiceEmployer UserRole = "service_employer" // Сотрудник сервисного центра
	UserRole_Admin           UserRole = "admin"            // Администратор
	UserRole_SuperAdmin      UserRole = "super_admin"      // Супер-администратор
	UserRole_Microservice    UserRole = "microservice"     // Для межсервисного взаимодействия
)

// User представляет базовую структуру пользователя для авторизации
type User struct {
	ID         uint     `json:"id"`
	Username   string   `json:"username"`
	FullName   string   `json:"full_name"`
	IsActive   bool     `json:"is_active"`
	Role       UserRole `json:"role"`
	TelegramID *int64   `json:"telegram_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Permission определяет уровни доступа для операций
type Permission string

const (
	// Базовые разрешения
	PermissionRead   Permission = "read"   // Чтение данных
	PermissionWrite  Permission = "write"  // Запись данных
	PermissionDelete Permission = "delete" // Удаление данных
	PermissionAdmin  Permission = "admin"  // Администраторские операции
	
	// Специальные разрешения
	PermissionOwn Permission = "own" // Доступ только к собственным данным
)

// ResourceType определяет типы ресурсов для контроля доступа
type ResourceType string

const (
	ResourceTypeAny           ResourceType = "*"              // Любой ресурс
	ResourceTypeUser          ResourceType = "user"           // Пользователи
	ResourceTypeOrder         ResourceType = "order"          // Заказы
	ResourceTypeServiceCenter ResourceType = "service_center" // Сервисные центры
	ResourceTypeDevice        ResourceType = "device"         // Устройства
	ResourceTypeReview        ResourceType = "review"         // Отзывы
)

// AuthContext содержит контекст авторизации для операций
type AuthContext struct {
	User      *User  // Пользователь, выполняющий операцию
	UserID    uint   // ID пользователя (для быстрого доступа)
	UserRole  UserRole // Роль пользователя (для быстрого доступа)
	IsAdmin   bool   // Флаг администраторских прав
	OwnerID   *uint  // ID владельца ресурса (если применимо)
}

// PermissionCheck определяет проверку разрешений
type PermissionCheck struct {
	Resource   ResourceType // Тип ресурса
	Permission Permission   // Требуемое разрешение
	OwnerField string       // Поле владельца в модели (например, "user_id")
}

// IsAdmin проверяет, является ли пользователь администратором
func (u *User) IsAdmin() bool {
	return u.Role == UserRole_Admin || u.Role == UserRole_SuperAdmin
}

// IsServiceOwner проверяет, является ли пользователь владельцем сервисного центра
func (u *User) IsServiceOwner() bool {
	return u.Role == UserRole_ServiceOwner || u.IsAdmin()
}

// IsServiceEmployee проверяет, является ли пользователь сотрудником сервисного центра
func (u *User) IsServiceEmployee() bool {
	return u.Role == UserRole_ServiceEmployer || u.IsServiceOwner()
}

// CanAccess проверяет, может ли пользователь получить доступ к ресурсу
func (u *User) CanAccess(resource ResourceType, permission Permission) bool {
	// Супер-админ имеет доступ ко всему
	if u.Role == UserRole_SuperAdmin {
		return true
	}
	
	// Администраторы имеют полный доступ, кроме некоторых супер-админских операций
	if u.Role == UserRole_Admin {
		return permission != Permission("super_admin")
	}
	
	// Проверяем доступ по ролям и ресурсам
	switch resource {
	case ResourceTypeUser:
		// Только админы могут управлять пользователями
		return u.IsAdmin()
		
	case ResourceTypeServiceCenter:
		// Владельцы сервисных центров и админы
		return u.IsServiceOwner() && (permission == PermissionRead || permission == PermissionWrite)
		
	case ResourceTypeOrder:
		// Все авторизованные пользователи могут читать заказы
		// Запись/удаление зависит от владения заказом
		return permission == PermissionRead || permission == PermissionOwn
		
	case ResourceTypeDevice, ResourceTypeReview:
		// Чтение доступно всем, запись - владельцам и админам
		return permission == PermissionRead || u.IsServiceEmployee()
		
	default:
		// По умолчанию только чтение для обычных пользователей
		return permission == PermissionRead
	}
}

// NewAuthContext создает новый контекст авторизации
func NewAuthContext(user *User) *AuthContext {
	if user == nil {
		return nil
	}
	
	return &AuthContext{
		User:     user,
		UserID:   user.ID,
		UserRole: user.Role,
		IsAdmin:  user.IsAdmin(),
	}
}

// WithOwner добавляет информацию о владельце к контексту авторизации
func (ac *AuthContext) WithOwner(ownerID uint) *AuthContext {
	if ac == nil {
		return nil
	}
	
	newContext := *ac
	newContext.OwnerID = &ownerID
	return &newContext
}

// CanPerform проверяет, может ли пользователь выполнить операцию
func (ac *AuthContext) CanPerform(check PermissionCheck) bool {
	if ac == nil || ac.User == nil {
		return false
	}
	
	// Проверяем базовые разрешения
	if !ac.User.CanAccess(check.Resource, check.Permission) {
		return false
	}
	
	// Если требуется проверка владения
	if check.Permission == PermissionOwn {
		// Админы могут получить доступ к любым данным
		if ac.IsAdmin {
			return true
		}
		
		// Проверяем, является ли пользователь владельцем
		if ac.OwnerID != nil {
			return ac.UserID == *ac.OwnerID
		}
		
		// Если информация о владельце не предоставлена, отказываем в доступе
		return false
	}
	
	return true
}