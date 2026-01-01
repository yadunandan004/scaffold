package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	EnvLocal      = "LOCAL"
	EnvStage      = "STAGE"
	EnvProduction = "PRODUCTION"
	EnvConfigPath = "CONFIG_PATH"
)

var hostingEnv string

type ServerConfig struct {
	HTTP HTTPConfig `yaml:"http"`
	GRPC GRPCConfig `yaml:"grpc"`
	TLS  TLSConfig  `yaml:"tls"`
}

type HTTPConfig struct {
	Port         string `yaml:"port"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
}

type GRPCConfig struct {
	Port              string `yaml:"port"`
	MaxConnectionIdle int    `yaml:"max_connection_idle"`
	MaxConnectionAge  int    `yaml:"max_connection_age"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type AuthConfig struct {
	JWTSecret            string `yaml:"jwt_secret"`
	PublicKey            string `yaml:"public_key"`
	PrivateKey           string `yaml:"private_key"`
	AccessTokenDuration  int    `yaml:"access_token_duration"`
	RefreshTokenDuration int    `yaml:"refresh_token_duration"`
}

type LoggerConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	OutputPath string `yaml:"output_path"`
}

type RedisConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	MaxRetries   int    `yaml:"max_retries"`
	DialTimeout  int    `yaml:"dial_timeout"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
	PoolSize     int    `yaml:"pool_size"`
}

func GetServerConfig(resolver *ConfigResolver) *ServerConfig {
	return &ServerConfig{
		HTTP: HTTPConfig{
			Port:         resolver.GetString("server.http.port", "HTTP_PORT", "8080"),
			ReadTimeout:  resolver.GetInt("server.http.read_timeout", "HTTP_READ_TIMEOUT", 30),
			WriteTimeout: resolver.GetInt("server.http.write_timeout", "HTTP_WRITE_TIMEOUT", 30),
		},
		GRPC: GRPCConfig{
			Port:              resolver.GetString("server.grpc.port", "GRPC_PORT", "9090"),
			MaxConnectionIdle: resolver.GetInt("server.grpc.max_connection_idle", "GRPC_MAX_CONN_IDLE", 300),
			MaxConnectionAge:  resolver.GetInt("server.grpc.max_connection_age", "GRPC_MAX_CONN_AGE", 600),
		},
		TLS: TLSConfig{
			Enabled:  resolver.GetBool("server.tls.enabled", "TLS_ENABLED", false),
			CertFile: resolver.GetString("server.tls.cert_file", "TLS_CERT_FILE", "/app/certs/server.crt"),
			KeyFile:  resolver.GetString("server.tls.key_file", "TLS_KEY_FILE", "/app/certs/server.key"),
		},
	}
}

func GetAuthConfig(resolver *ConfigResolver) *AuthConfig {
	return &AuthConfig{
		JWTSecret:            resolver.GetString("auth.jwt_secret", "JWT_SECRET", ""),
		PublicKey:            resolver.GetString("auth.public_key", "JWT_PUBLIC_KEY", ""),
		PrivateKey:           resolver.GetString("auth.private_key", "JWT_PRIVATE_KEY", ""),
		AccessTokenDuration:  resolver.GetInt("auth.access_token_duration", "ACCESS_TOKEN_DURATION", 3600),
		RefreshTokenDuration: resolver.GetInt("auth.refresh_token_duration", "REFRESH_TOKEN_DURATION", 7776000),
	}
}

func GetLoggerConfig(resolver *ConfigResolver) *LoggerConfig {
	return &LoggerConfig{
		Level:      resolver.GetString("logger.level", "LOG_LEVEL", "info"),
		Format:     resolver.GetString("logger.format", "LOG_FORMAT", "console"),
		OutputPath: resolver.GetString("logger.output_path", "LOG_OUTPUT_PATH", ""),
	}
}

func GetRedisConfig(resolver *ConfigResolver) *RedisConfig {
	return &RedisConfig{
		Enabled:      resolver.GetBool("redis.enabled", "REDIS_ENABLED", false),
		Host:         resolver.GetString("redis.host", "REDIS_HOST", "localhost"),
		Port:         resolver.GetInt("redis.port", "REDIS_PORT", 6379),
		Password:     resolver.GetString("redis.password", "REDIS_PASSWORD", ""),
		DB:           resolver.GetInt("redis.db", "REDIS_DB", 0),
		MaxRetries:   resolver.GetInt("redis.max_retries", "REDIS_MAX_RETRIES", 3),
		DialTimeout:  resolver.GetInt("redis.dial_timeout", "REDIS_DIAL_TIMEOUT", 5),
		ReadTimeout:  resolver.GetInt("redis.read_timeout", "REDIS_READ_TIMEOUT", 3),
		WriteTimeout: resolver.GetInt("redis.write_timeout", "REDIS_WRITE_TIMEOUT", 3),
		PoolSize:     resolver.GetInt("redis.pool_size", "REDIS_POOL_SIZE", 10),
	}
}

func LoadConfigFile(configPath string) (map[string]interface{}, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

func GetDefaultConfigPath() string {
	if path := os.Getenv(EnvConfigPath); path != "" {
		return path
	}

	locations := []string{
		"config.yaml",
		"config/config.yaml",
		"/etc/app/config.yaml",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return "config.yaml"
}

func InitHostingEnv() {
	hostingEnv = os.Getenv("HOSTING_ENV")
	if hostingEnv == "" {
		hostingEnv = EnvLocal
	}
}

func IsLocalEnv() bool {
	return hostingEnv == EnvLocal
}

func IsStageEnv() bool {
	return hostingEnv == EnvStage
}

func IsProductionEnv() bool {
	return hostingEnv == EnvProduction
}

func GetHostingEnv() string {
	return hostingEnv
}
