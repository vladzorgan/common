package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Ключи для хранения данных в контексте
type contextKey string

const (
	UserContextKey     contextKey = "user"
	AuthContextKey     contextKey = "auth_context"
	UserIDContextKey   contextKey = "user_id"
	UserRoleContextKey contextKey = "user_role"
)

// UserProvider определяет интерфейс для получения пользователя по ID
type UserProvider interface {
	GetUserByID(ctx context.Context, userID uint) (*User, error)
}

// ContextManager управляет авторизационным контекстом
type ContextManager struct {
	userProvider UserProvider
}

// NewContextManager создает новый менеджер контекста
func NewContextManager(userProvider UserProvider) *ContextManager {
	return &ContextManager{
		userProvider: userProvider,
	}
}

// ExtractUserFromMetadata извлекает информацию о пользователе из gRPC метаданных
func (cm *ContextManager) ExtractUserFromMetadata(ctx context.Context) (*User, error) {
	// Получаем метаданные из контекста
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("не удалось получить метаданные из контекста")
	}

	// Извлекаем user-id
	userIDValues := md.Get("user-id")
	if len(userIDValues) == 0 {
		return nil, errors.New("пользователь не авторизован: отсутствует user-id")
	}

	// Парсим user-id
	userID, err := strconv.ParseUint(userIDValues[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("неверный формат ID пользователя: %w", err)
	}

	// Получаем пользователя из базы данных через провайдер
	if cm.userProvider != nil {
		user, err := cm.userProvider.GetUserByID(ctx, uint(userID))
		if err != nil {
			return nil, fmt.Errorf("ошибка при получении пользователя: %w", err)
		}

		if user == nil {
			return nil, errors.New("пользователь не найден")
		}

		return user, nil
	}

	// Если провайдер не установлен, создаем базовую структуру пользователя из метаданных
	user := &User{
		ID: uint(userID),
	}

	// Пытаемся извлечь роль из метаданных
	roleValues := md.Get("user-role")
	if len(roleValues) > 0 {
		user.Role = UserRole(roleValues[0])
	}

	return user, nil
}

// GetUserFromContext получает пользователя из контекста
func GetUserFromContext(ctx context.Context) (*User, error) {
	user, ok := ctx.Value(UserContextKey).(*User)
	if !ok {
		return nil, errors.New("пользователь не найден в контексте")
	}
	
	if user == nil {
		return nil, errors.New("пользователь равен nil")
	}
	
	return user, nil
}

// GetAuthContextFromContext получает авторизационный контекст
func GetAuthContextFromContext(ctx context.Context) (*AuthContext, error) {
	authCtx, ok := ctx.Value(AuthContextKey).(*AuthContext)
	if !ok {
		// Пробуем создать авторизационный контекст из пользователя
		user, err := GetUserFromContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("авторизационный контекст не найден: %w", err)
		}
		
		return NewAuthContext(user), nil
	}
	
	if authCtx == nil {
		return nil, errors.New("авторизационный контекст равен nil")
	}
	
	return authCtx, nil
}

// GetUserIDFromContext получает ID пользователя из контекста
func GetUserIDFromContext(ctx context.Context) (uint, error) {
	// Сначала пробуем получить из прямого значения
	if userID, ok := ctx.Value(UserIDContextKey).(uint); ok {
		return userID, nil
	}
	
	// Если не найдено, получаем из пользователя
	user, err := GetUserFromContext(ctx)
	if err != nil {
		return 0, err
	}
	
	return user.ID, nil
}

// GetUserRoleFromContext получает роль пользователя из контекста
func GetUserRoleFromContext(ctx context.Context) (UserRole, error) {
	// Сначала пробуем получить из прямого значения
	if userRole, ok := ctx.Value(UserRoleContextKey).(UserRole); ok {
		return userRole, nil
	}
	
	// Если не найдено, получаем из пользователя
	user, err := GetUserFromContext(ctx)
	if err != nil {
		return "", err
	}
	
	return user.Role, nil
}

// WithUser добавляет пользователя в контекст
func WithUser(ctx context.Context, user *User) context.Context {
	ctx = context.WithValue(ctx, UserContextKey, user)
	ctx = context.WithValue(ctx, UserIDContextKey, user.ID)
	ctx = context.WithValue(ctx, UserRoleContextKey, user.Role)
	
	// Создаем и добавляем авторизационный контекст
	authCtx := NewAuthContext(user)
	ctx = context.WithValue(ctx, AuthContextKey, authCtx)
	
	return ctx
}

// WithAuthContext добавляет авторизационный контекст
func WithAuthContext(ctx context.Context, authCtx *AuthContext) context.Context {
	ctx = context.WithValue(ctx, AuthContextKey, authCtx)
	
	if authCtx != nil && authCtx.User != nil {
		ctx = context.WithValue(ctx, UserContextKey, authCtx.User)
		ctx = context.WithValue(ctx, UserIDContextKey, authCtx.UserID)
		ctx = context.WithValue(ctx, UserRoleContextKey, authCtx.UserRole)
	}
	
	return ctx
}

// RequireAuth проверяет, что пользователь авторизован
func RequireAuth(ctx context.Context) (*User, error) {
	user, err := GetUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "Требуется авторизация: %v", err)
	}
	
	if !user.IsActive {
		return nil, status.Errorf(codes.Unauthenticated, "Пользователь неактивен")
	}
	
	return user, nil
}

// RequireAdmin проверяет, что пользователь является администратором
func RequireAdmin(ctx context.Context) (*User, error) {
	user, err := RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	
	if !user.IsAdmin() {
		return nil, status.Errorf(codes.PermissionDenied, "Требуются права администратора")
	}
	
	return user, nil
}

// RequireServiceOwner проверяет, что пользователь является владельцем сервисного центра
func RequireServiceOwner(ctx context.Context) (*User, error) {
	user, err := RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	
	if !user.IsServiceOwner() {
		return nil, status.Errorf(codes.PermissionDenied, "Требуются права владельца сервисного центра")
	}
	
	return user, nil
}

// RequireRole проверяет, что пользователь имеет определенную роль
func RequireRole(ctx context.Context, requiredRole UserRole) (*User, error) {
	user, err := RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	
	if user.Role != requiredRole && !user.IsAdmin() {
		return nil, status.Errorf(codes.PermissionDenied, "Требуется роль: %s", requiredRole)
	}
	
	return user, nil
}

// RequirePermission проверяет, что пользователь имеет разрешение на операцию
func RequirePermission(ctx context.Context, check PermissionCheck) (*AuthContext, error) {
	authCtx, err := GetAuthContextFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "Требуется авторизация: %v", err)
	}
	
	if !authCtx.CanPerform(check) {
		return nil, status.Errorf(codes.PermissionDenied, 
			"Недостаточно прав для операции %s на ресурсе %s", 
			check.Permission, check.Resource)
	}
	
	return authCtx, nil
}

// CheckOwnership проверяет, что пользователь является владельцем ресурса
func CheckOwnership(ctx context.Context, ownerID uint) error {
	user, err := RequireAuth(ctx)
	if err != nil {
		return err
	}
	
	// Админы имеют доступ к любым ресурсам
	if user.IsAdmin() {
		return nil
	}
	
	// Проверяем владение
	if user.ID != ownerID {
		return status.Errorf(codes.PermissionDenied, "Доступ запрещен: недостаточно прав")
	}
	
	return nil
}

// IsOwner проверяет, является ли пользователь владельцем ресурса (без ошибки)
func IsOwner(ctx context.Context, ownerID uint) bool {
	user, err := GetUserFromContext(ctx)
	if err != nil {
		return false
	}
	
	// Админы считаются владельцами всех ресурсов
	if user.IsAdmin() {
		return true
	}
	
	return user.ID == ownerID
}