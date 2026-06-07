package config

import "github.com/kelseyhightower/envconfig"

type APIConfig struct {
	Port           string `env:"PORT" default:"8080"`
	GRPCPort       string `env:"GRPC_PORT" default:"9090"`
	DatabaseURL    string `env:"DATABASE_URL" required:"true" split_words:"true"`
	MigrationsPath string `env:"MIGRATIONS_PATH" default:"./internal/db/migrations" split_words:"true"`
}

func NewAPIConfig() *APIConfig {
	config := &APIConfig{}
	err := envconfig.Process("", config)
	if err != nil {
		panic(err)
	}

	return config
}
