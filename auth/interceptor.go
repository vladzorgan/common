package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthInterceptor представляет интерцептор для авторизации
type AuthInterceptor struct {
	contextManager *ContextManager
	skipMethods    map[string]bool // Методы, которые не требуют авторизации
}

// NewAuthInterceptor создает новый интерцептор авторизации
func NewAuthInterceptor(userProvider UserProvider, skipMethods []string) *AuthInterceptor {
	skipMap := make(map[string]bool)
	for _, method := range skipMethods {
		skipMap[method] = true
	}

	return &AuthInterceptor{
		contextManager: NewContextManager(userProvider),
		skipMethods:    skipMap,
	}
}

// UnaryInterceptor возвращает unary интерцептор для авторизации
func (ai *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Проверяем, нужна ли авторизация для этого метода
		if ai.skipMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		// Извлекаем пользователя из метаданных
		user, err := ai.contextManager.ExtractUserFromMetadata(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "Ошибка авторизации: %v", err)
		}

		// Добавляем пользователя в контекст
		ctx = WithUser(ctx, user)

		// Вызываем обработчик с обновленным контекстом
		return handler(ctx, req)
	}
}

// StreamInterceptor возвращает stream интерцептор для авторизации
func (ai *AuthInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Проверяем, нужна ли авторизация для этого метода
		if ai.skipMethods[info.FullMethod] {
			return handler(srv, ss)
		}

		// Извлекаем пользователя из метаданных
		user, err := ai.contextManager.ExtractUserFromMetadata(ss.Context())
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "Ошибка авторизации: %v", err)
		}

		// Создаем обертку для stream с обновленным контекстом
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          WithUser(ss.Context(), user),
		}

		// Вызываем обработчик с обновленным stream
		return handler(srv, wrappedStream)
	}
}

// wrappedServerStream обертка для ServerStream с обновленным контекстом
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// DefaultSkipMethods возвращает список методов, которые обычно не требуют авторизации
func DefaultSkipMethods() []string {
	return []string{
		"/grpc.health.v1.Health/Check",
		"/grpc.health.v1.Health/Watch",
		"/auth.AuthService/Login",
		"/auth.AuthService/Register",
		"/auth.AuthService/RefreshToken",
		"/auth.AuthService/RequestPasswordReset",
		"/auth.AuthService/ResetPassword",
		"/auth.AuthService/TelegramAuth",
		"/auth.AuthService/LoginOAuth",
	}
}

// ExtractBearerToken извлекает Bearer токен из метаданных
func ExtractBearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "отсутствуют метаданные")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return "", status.Error(codes.Unauthenticated, "отсутствует заголовок авторизации")
	}

	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", status.Error(codes.Unauthenticated, "неверный формат заголовка авторизации")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return "", status.Error(codes.Unauthenticated, "пустой токен")
	}

	return token, nil
}

// RequireAuthInterceptor создает интерцептор, который требует авторизацию для всех методов
func RequireAuthInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Проверяем авторизацию
		_, err := RequireAuth(ctx)
		if err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// RequireAdminInterceptor создает интерцептор, который требует права администратора
func RequireAdminInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Проверяем права администратора
		_, err := RequireAdmin(ctx)
		if err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// RequireRoleInterceptor создает интерцептор, который требует определенную роль
func RequireRoleInterceptor(role UserRole) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Проверяем роль
		_, err := RequireRole(ctx, role)
		if err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}