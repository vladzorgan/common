// Package interceptors предоставляет набор интерцепторов для gRPC сервера
package interceptors

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rem-consultant/common/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Унарные интерцепторы

// ChainUnaryInterceptors объединяет несколько унарных интерцепторов в один
func ChainUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		buildChain := func(current grpc.UnaryServerInterceptor, next grpc.UnaryHandler) grpc.UnaryHandler {
			return func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
				return current(currentCtx, currentReq, info, next)
			}
		}

		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			chain = buildChain(interceptors[i], chain)
		}

		return chain(ctx, req)
	}
}

// LoggingUnaryInterceptor создает интерцептор для логирования унарных запросов
func LoggingUnaryInterceptor(logger logging.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()

		// Извлекаем метаданные и peer информацию
		md, _ := metadata.FromIncomingContext(ctx)
		peer, _ := peer.FromContext(ctx)

		// Получаем request ID из метаданных или генерируем новый
		requestID := ""
		if requestIDs := md.Get("x-request-id"); len(requestIDs) > 0 {
			requestID = requestIDs[0]
		}

		if requestID == "" {
			requestID = logging.GenerateRequestID()
			// Добавляем request ID в исходящие метаданные
			ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", requestID)
		}

		// Обогащаем контекст request ID
		ctx = logging.ContextWithRequestID(ctx, requestID)

		// Создаем логгер с контекстом запроса
		reqLogger := logger.WithRequestID(requestID).
			WithField("method", info.FullMethod).
			WithField("peer_addr", peer.Addr.String())

		reqLogger.Info("gRPC request started")

		// Вызываем обработчик
		resp, err := handler(ctx, req)

		// Логируем результат и время выполнения
		duration := time.Since(startTime)
		statusCode := status.Code(err)

		logFields := map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
			"status":      statusCode.String(),
		}

		if err != nil {
			logFields["error"] = err.Error()
			reqLogger.WithFields(logFields).Error("gRPC request failed")
		} else {
			reqLogger.WithFields(logFields).Info("gRPC request completed")
		}

		return resp, err
	}
}

// RecoveryUnaryInterceptor создает интерцептор для восстановления после паники в унарных запросах
func RecoveryUnaryInterceptor(logger logging.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		defer func() {
			if r := recover(); r != nil {
				stackTrace := string(debug.Stack())

				// Получаем request ID из контекста
				requestID := logging.ExtractRequestID(ctx)

				// Логируем панику
				logger.WithRequestID(requestID).
					WithField("method", info.FullMethod).
					WithField("stack", stackTrace).
					Error("Panic recovered in gRPC handler: %v", r)

				// Возвращаем Internal Server Error
				err := status.Errorf(codes.Internal, "Internal server error")
				panic(err) // Перепаникуем с правильной gRPC ошибкой
			}
		}()

		return handler(ctx, req)
	}
}

// MetricsUnaryInterceptor создает интерцептор для сбора метрик унарных запросов
func MetricsUnaryInterceptor(servicePrefix string) grpc.UnaryServerInterceptor {
	// Создаем счетчики и гистограммы для метрик
	requestsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: servicePrefix + "_grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    servicePrefix + "_grpc_request_duration_ms",
			Help:    "gRPC request duration in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15), // От 1мс до ~16с
		},
		[]string{"method", "status"},
	)

	// Регистрируем метрики
	prometheus.MustRegister(requestsCounter, requestDuration)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()

		// Вызываем обработчик
		resp, err := handler(ctx, req)

		// Обновляем метрики
		duration := time.Since(startTime)
		statusCode := status.Code(err)

		requestsCounter.WithLabelValues(info.FullMethod, statusCode.String()).Inc()
		requestDuration.WithLabelValues(info.FullMethod, statusCode.String()).Observe(float64(duration.Milliseconds()))

		return resp, err
	}
}

// AuthUnaryInterceptor создает интерцептор для аутентификации унарных запросов
func AuthUnaryInterceptor(apiKey string, excludedMethods []string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Проверяем, входит ли метод в список исключений
		for _, excludedMethod := range excludedMethods {
			if info.FullMethod == excludedMethod {
				return handler(ctx, req)
			}
		}

		// Получаем API ключ из метаданных
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		// Проверяем наличие API ключа
		values := md.Get("x-api-key")
		if len(values) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing API key")
		}

		// Проверяем значение API ключа
		if values[0] != apiKey {
			return nil, status.Errorf(codes.Unauthenticated, "invalid API key")
		}

		return handler(ctx, req)
	}
}

// Потоковые интерцепторы

// ChainStreamInterceptors объединяет несколько потоковых интерцепторов в один
func ChainStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		buildChain := func(current grpc.StreamServerInterceptor, next grpc.StreamHandler) grpc.StreamHandler {
			return func(currentSrv interface{}, currentStream grpc.ServerStream) error {
				return current(currentSrv, currentStream, info, next)
			}
		}

		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			chain = buildChain(interceptors[i], chain)
		}

		return chain(srv, ss)
	}
}

// LoggingStreamInterceptor создает интерцептор для логирования потоковых запросов
func LoggingStreamInterceptor(logger logging.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()

		// Получаем метаданные и peer информацию
		ctx := ss.Context()
		md, _ := metadata.FromIncomingContext(ctx)
		peer, _ := peer.FromContext(ctx)

		// Получаем request ID из метаданных или генерируем новый
		requestID := ""
		if requestIDs := md.Get("x-request-id"); len(requestIDs) > 0 {
			requestID = requestIDs[0]
		}

		if requestID == "" {
			requestID = logging.GenerateRequestID()
		}

		// Создаем обертку для ServerStream с дополнительным контекстом
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          logging.ContextWithRequestID(ctx, requestID),
		}

		// Создаем логгер с контекстом запроса
		reqLogger := logger.WithRequestID(requestID).
			WithField("method", info.FullMethod).
			WithField("peer_addr", peer.Addr.String()).
			WithField("stream_type", streamTypeFromInfo(info))

		reqLogger.Info("gRPC stream started")

		// Вызываем обработчик
		err := handler(srv, wrappedStream)

		// Логируем результат и время выполнения
		duration := time.Since(startTime)
		statusCode := status.Code(err)

		logFields := map[string]interface{}{
			"duration_ms": duration.Milliseconds(),
			"status":      statusCode.String(),
		}

		if err != nil {
			logFields["error"] = err.Error()
			reqLogger.WithFields(logFields).Error("gRPC stream failed")
		} else {
			reqLogger.WithFields(logFields).Info("gRPC stream completed")
		}

		return err
	}
}

// RecoveryStreamInterceptor создает интерцептор для восстановления после паники в потоковых запросах
func RecoveryStreamInterceptor(logger logging.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		defer func() {
			if r := recover(); r != nil {
				stackTrace := string(debug.Stack())

				// Получаем request ID из контекста
				ctx := ss.Context()
				requestID := logging.ExtractRequestID(ctx)

				// Логируем панику
				logger.WithRequestID(requestID).
					WithField("method", info.FullMethod).
					WithField("stack", stackTrace).
					Error("Panic recovered in gRPC stream handler: %v", r)

				// Возвращаем Internal Server Error
				err := status.Errorf(codes.Internal, "Internal server error")
				panic(err) // Перепаникуем с правильной gRPC ошибкой
			}
		}()

		return handler(srv, ss)
	}
}

// MetricsStreamInterceptor создает интерцептор для сбора метрик потоковых запросов
func MetricsStreamInterceptor(servicePrefix string) grpc.StreamServerInterceptor {
	// Создаем счетчики и гистограммы для метрик
	streamsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: servicePrefix + "_grpc_streams_total",
			Help: "Total number of gRPC streams",
		},
		[]string{"method", "stream_type", "status"},
	)

	streamDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    servicePrefix + "_grpc_stream_duration_ms",
			Help:    "gRPC stream duration in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15), // От 1мс до ~16с
		},
		[]string{"method", "stream_type", "status"},
	)

	// Регистрируем метрики
	prometheus.MustRegister(streamsCounter, streamDuration)

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		streamType := streamTypeFromInfo(info)

		// Вызываем обработчик
		err := handler(srv, ss)

		// Обновляем метрики
		duration := time.Since(startTime)
		statusCode := status.Code(err)

		streamsCounter.WithLabelValues(info.FullMethod, streamType, statusCode.String()).Inc()
		streamDuration.WithLabelValues(info.FullMethod, streamType, statusCode.String()).Observe(float64(duration.Milliseconds()))

		return err
	}
}

// AuthStreamInterceptor создает интерцептор для аутентификации потоковых запросов
func AuthStreamInterceptor(apiKey string, excludedMethods []string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Проверяем, входит ли метод в список исключений
		for _, excludedMethod := range excludedMethods {
			if info.FullMethod == excludedMethod {
				return handler(srv, ss)
			}
		}

		// Получаем API ключ из метаданных
		ctx := ss.Context()
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		// Проверяем наличие API ключа
		values := md.Get("x-api-key")
		if len(values) == 0 {
			return status.Errorf(codes.Unauthenticated, "missing API key")
		}

		// Проверяем значение API ключа
		if values[0] != apiKey {
			return status.Errorf(codes.Unauthenticated, "invalid API key")
		}

		return handler(srv, ss)
	}
}

// Вспомогательные функции и типы

// wrappedServerStream оборачивает grpc.ServerStream для передачи обогащенного контекста
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context возвращает обогащенный контекст
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// streamTypeFromInfo возвращает тип потока на основе информации о методе
func streamTypeFromInfo(info *grpc.StreamServerInfo) string {
	if info.IsClientStream && info.IsServerStream {
		return "bidi_stream"
	} else if info.IsClientStream {
		return "client_stream"
	} else if info.IsServerStream {
		return "server_stream"
	}
	return "unary" // Не должны попасть сюда для потоковых запросов
}
