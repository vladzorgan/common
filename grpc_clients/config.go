package grpc_clients

import "time"

// DefaultConfig создает конфигурацию с настройками по умолчанию
func DefaultConfig() *Config {
	return &Config{
		Services: make(map[string]ServiceConfigBase),
	}
}

// DefaultLocationConfig создает конфигурацию только для location-service
func DefaultLocationConfig() *Config {
	return &Config{
		Services: map[string]ServiceConfigBase{
			"location-service": {
				Address:     "location-service",
				Port:        "50053",
				Timeout:     10 * time.Second,
				MaxRetries:  3,
				HealthCheck: true,
			},
		},
	}
}

// AddLocationService добавляет location-service в конфигурацию
func (c *Config) AddLocationService() *Config {
	c.Services["location-service"] = ServiceConfigBase{
		Address:     "location-service",
		Port:        "50053",
		Timeout:     10 * time.Second,
		MaxRetries:  3,
		HealthCheck: true,
	}
	return c
}