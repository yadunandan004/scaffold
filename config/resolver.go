package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigResolver provides unified configuration resolution with precedence:
// 1. Config file values
// 2. Environment variables
// 3. Default values
type ConfigResolver struct {
	configData map[string]interface{} // Loaded config file data
	envPrefix  string                 // Optional prefix for environment variables
}

// NewConfigResolver creates a new configuration resolver
func NewConfigResolver(configPath string) *ConfigResolver {
	resolver := &ConfigResolver{
		envPrefix: "", // No prefix by default, can be added later if needed
	}

	// Try to load config file first (optional)
	if configPath != "" {
		if data, err := loadConfigFile(configPath); err == nil {
			resolver.configData = data
		}
		// Silently continue if config file doesn't exist - env vars and defaults will be used
	}

	return resolver
}

// NewConfigResolverWithPrefix creates a resolver with an environment variable prefix
func NewConfigResolverWithPrefix(configPath, envPrefix string) *ConfigResolver {
	resolver := NewConfigResolver(configPath)
	resolver.envPrefix = envPrefix
	return resolver
}

// GetString resolves a string configuration value with precedence: file → env → default
func (cr *ConfigResolver) GetString(configKey, envKey, defaultValue string) string {
	// 1st Priority: Config file
	if cr.configData != nil {
		if value, exists := cr.getNestedValue(configKey); exists {
			if str, ok := cr.toString(value); ok {
				return str
			}
		}
	}

	// 2nd Priority: Environment variable (with optional prefix)
	envVarName := cr.buildEnvVarName(envKey)
	if envValue := os.Getenv(envVarName); envValue != "" {
		return envValue
	}

	// 3rd Priority: Default value
	return defaultValue
}

// GetInt resolves an integer configuration value with precedence: file → env → default
func (cr *ConfigResolver) GetInt(configKey, envKey string, defaultValue int) int {
	// 1st Priority: Config file
	if cr.configData != nil {
		if value, exists := cr.getNestedValue(configKey); exists {
			if intVal, ok := cr.toInt(value); ok {
				return intVal
			}
		}
	}

	// 2nd Priority: Environment variable
	envVarName := cr.buildEnvVarName(envKey)
	if envValue := os.Getenv(envVarName); envValue != "" {
		if parsed, err := strconv.Atoi(envValue); err == nil {
			return parsed
		}
	}

	// 3rd Priority: Default value
	return defaultValue
}

// GetBool resolves a boolean configuration value with precedence: file → env → default
func (cr *ConfigResolver) GetBool(configKey, envKey string, defaultValue bool) bool {
	// 1st Priority: Config file
	if cr.configData != nil {
		if value, exists := cr.getNestedValue(configKey); exists {
			if boolVal, ok := cr.toBool(value); ok {
				return boolVal
			}
		}
	}

	// 2nd Priority: Environment variable
	envVarName := cr.buildEnvVarName(envKey)
	if envValue := os.Getenv(envVarName); envValue != "" {
		// Parse common boolean representations
		switch strings.ToLower(envValue) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}

	// 3rd Priority: Default value
	return defaultValue
}

// GetStringSlice resolves a string slice configuration value
func (cr *ConfigResolver) GetStringSlice(configKey, envKey string, defaultValue []string) []string {
	// 1st Priority: Config file
	if cr.configData != nil {
		if value, exists := cr.getNestedValue(configKey); exists {
			if slice, ok := cr.toStringSlice(value); ok {
				return slice
			}
		}
	}

	// 2nd Priority: Environment variable (comma-separated)
	envVarName := cr.buildEnvVarName(envKey)
	if envValue := os.Getenv(envVarName); envValue != "" {
		return strings.Split(envValue, ",")
	}

	// 3rd Priority: Default value
	return defaultValue
}

// HasConfigFile returns true if a config file was successfully loaded
func (cr *ConfigResolver) HasConfigFile() bool {
	return cr.configData != nil
}

// GetLoadedConfigKeys returns all keys present in the loaded config file (for debugging)
func (cr *ConfigResolver) GetLoadedConfigKeys() []string {
	if cr.configData == nil {
		return nil
	}
	return cr.extractKeys(cr.configData, "")
}

// Private helper methods

func (cr *ConfigResolver) buildEnvVarName(envKey string) string {
	if cr.envPrefix == "" {
		return envKey
	}
	return cr.envPrefix + envKey
}

func (cr *ConfigResolver) getNestedValue(key string) (interface{}, bool) {
	if cr.configData == nil {
		return nil, false
	}

	parts := strings.Split(key, ".")
	current := cr.configData

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - get the value
			if value, exists := current[part]; exists {
				return value, true
			}
			return nil, false
		}

		// Navigate to next level
		if next, exists := current[part]; exists {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return nil, false
			}
		} else {
			return nil, false
		}
	}

	return nil, false
}

func (cr *ConfigResolver) toString(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(v), true
	default:
		return "", false
	}
}

func (cr *ConfigResolver) toInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func (cr *ConfigResolver) toBool(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(v) {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		}
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	}
	return false, false
}

func (cr *ConfigResolver) toStringSlice(value interface{}) ([]string, bool) {
	switch v := value.(type) {
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			if str, ok := cr.toString(item); ok {
				result[i] = str
			} else {
				return nil, false
			}
		}
		return result, true
	case []string:
		return v, true
	case string:
		// Single string becomes single-item slice
		return []string{v}, true
	}
	return nil, false
}

func (cr *ConfigResolver) extractKeys(data map[string]interface{}, prefix string) []string {
	var keys []string
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		if subMap, ok := value.(map[string]interface{}); ok {
			// Recursively extract nested keys
			keys = append(keys, cr.extractKeys(subMap, fullKey)...)
		} else {
			keys = append(keys, fullKey)
		}
	}
	return keys
}

func loadConfigFile(configPath string) (map[string]interface{}, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", configPath)
	}

	// Read file content
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}
