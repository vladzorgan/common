package grpc_clients

import (
	"context"

	locationpb "github.com/vladzorgan/common/proto/location"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	LocationServiceName   = "location-service"
	LocationServiceURLKey = "location-service"
	LocationDefaultPort   = "50053"
)

// LocationClient представляет gRPC клиент для сервиса местоположений
type LocationClient struct {
	*BaseClient
	client locationpb.LocationServiceClient
}

// NewLocationClient создает новый клиент для сервиса местоположений
func NewLocationClient(cfg *Config) (*LocationClient, error) {
	// Настраиваем опции клиента
	options := DefaultOptions(LocationServiceName, LocationServiceURLKey, LocationDefaultPort)

	// Создаем базовый клиент
	baseClient, err := NewBaseClient(cfg, options)
	if err != nil {
		return nil, err
	}

	// Создаем gRPC клиент
	client := locationpb.NewLocationServiceClient(baseClient.Conn)

	return &LocationClient{
		BaseClient: baseClient,
		client:     client,
	}, nil
}

// Методы для работы с регионами

// GetRegion получает регион по ID
func (c *LocationClient) GetRegion(ctx context.Context, id uint32) (*locationpb.RegionResponse, error) {
	request := &locationpb.GetRegionRequest{Id: id}
	return MeasureCall(ctx, LocationServiceName, "GetRegion", request, c.client.GetRegion)
}

// GetRegions получает список регионов с пагинацией
func (c *LocationClient) GetRegions(ctx context.Context, skip, limit int32, sort *locationpb.SortOptions) (*locationpb.GetRegionsResponse, error) {
	request := &locationpb.GetRegionsRequest{
		Skip:  skip,
		Limit: limit,
		Sort:  sort,
	}
	return MeasureCall(ctx, LocationServiceName, "GetRegions", request, c.client.GetRegions)
}

// CreateRegion создает новый регион
func (c *LocationClient) CreateRegion(ctx context.Context, name, code, country string) (*locationpb.RegionResponse, error) {
	request := &locationpb.CreateRegionRequest{
		Name:    name,
		Code:    code,
		Country: country,
	}
	return MeasureCall(ctx, LocationServiceName, "CreateRegion", request, c.client.CreateRegion)
}

// UpdateRegion обновляет регион
func (c *LocationClient) UpdateRegion(ctx context.Context, id uint32, name, code, country string) (*locationpb.RegionResponse, error) {
	request := &locationpb.UpdateRegionRequest{
		Id:      id,
		Name:    name,
		Code:    code,
		Country: country,
	}
	return MeasureCall(ctx, LocationServiceName, "UpdateRegion", request, c.client.UpdateRegion)
}

// DeleteRegion удаляет регион
func (c *LocationClient) DeleteRegion(ctx context.Context, id uint32) (*locationpb.RegionResponse, error) {
	request := &locationpb.DeleteRegionRequest{Id: id}
	return MeasureCall(ctx, LocationServiceName, "DeleteRegion", request, c.client.DeleteRegion)
}

// Методы для работы с городами

// GetCity получает город по ID
func (c *LocationClient) GetCity(ctx context.Context, id uint32) (*locationpb.CityResponse, error) {
	request := &locationpb.GetCityRequest{Id: id}
	return MeasureCall(ctx, LocationServiceName, "GetCity", request, c.client.GetCity)
}

// GetCityBySlug получает город по slug
func (c *LocationClient) GetCityBySlug(ctx context.Context, slug string) (*locationpb.CityResponse, error) {
	request := &locationpb.GetCityBySlugRequest{Slug: slug}
	return MeasureCall(ctx, LocationServiceName, "GetCityBySlug", request, c.client.GetCityBySlug)
}

// GetCities получает список городов с фильтрацией и пагинацией
func (c *LocationClient) GetCities(ctx context.Context, skip, limit int32, filter *locationpb.CityFilter, sort *locationpb.SortOptions) (*locationpb.GetCitiesResponse, error) {
	request := &locationpb.GetCitiesRequest{
		Skip:   skip,
		Limit:  limit,
		Filter: filter,
		Sort:   sort,
	}
	return MeasureCall(ctx, LocationServiceName, "GetCities", request, c.client.GetCities)
}

// GetLargestCities получает самые крупные города
func (c *LocationClient) GetLargestCities(ctx context.Context, limit int32, sort *locationpb.SortOptions) (*locationpb.GetCitiesResponse, error) {
	request := &locationpb.GetLargestCitiesRequest{
		Limit: limit,
		Sort:  sort,
	}
	return MeasureCall(ctx, LocationServiceName, "GetLargestCities", request, c.client.GetLargestCities)
}

// CreateCity создает новый город
func (c *LocationClient) CreateCity(ctx context.Context, req *locationpb.CreateCityRequest) (*locationpb.CityResponse, error) {
	return MeasureCall(ctx, LocationServiceName, "CreateCity", req, c.client.CreateCity)
}

// UpdateCity обновляет город
func (c *LocationClient) UpdateCity(ctx context.Context, req *locationpb.UpdateCityRequest) (*locationpb.CityResponse, error) {
	return MeasureCall(ctx, LocationServiceName, "UpdateCity", req, c.client.UpdateCity)
}

// DeleteCity удаляет город
func (c *LocationClient) DeleteCity(ctx context.Context, id uint32) (*locationpb.CityResponse, error) {
	request := &locationpb.DeleteCityRequest{Id: id}
	return MeasureCall(ctx, LocationServiceName, "DeleteCity", request, c.client.DeleteCity)
}

// Методы для аналитики

// GetSearchStats получает статистику поиска
func (c *LocationClient) GetSearchStats(ctx context.Context) (*locationpb.SearchStatsResponse, error) {
	request := &emptypb.Empty{}
	return MeasureCall(ctx, LocationServiceName, "GetSearchStats", request, c.client.GetSearchStats)
}

// GetMostSearchedQueries получает самые популярные поисковые запросы
func (c *LocationClient) GetMostSearchedQueries(ctx context.Context, limit int32) (*locationpb.MostSearchedQueriesResponse, error) {
	request := &locationpb.GetMostSearchedQueriesRequest{Limit: limit}
	return MeasureCall(ctx, LocationServiceName, "GetMostSearchedQueries", request, c.client.GetMostSearchedQueries)
}