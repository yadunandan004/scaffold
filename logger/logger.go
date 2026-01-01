package logger

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/yadunandan004/scaffold/logger/logwriter"
	"github.com/yadunandan004/scaffold/request"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log    *zap.Logger
	writer logwriter.LogWriter
)

func init() {
	InitializeLogger()
}

// InitializeLogger sets up the logger based on environment configuration
func InitializeLogger() {
	// Get configuration from environment
	logFormat := getEnvOrDefault("LOG_FORMAT", "console")
	logLevel := getEnvOrDefault("LOG_LEVEL", "debug")
	logWriter := getEnvOrDefault("LOG_WRITER", "local")
	environment := getEnvOrDefault("ENV", "development")

	// Configure zap logger
	var config zapcore.EncoderConfig
	var encoder zapcore.Encoder

	if environment == "production" {
		config = zap.NewProductionEncoderConfig()
		config.TimeKey = "timestamp"
		config.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentEncoderConfig()
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
		config.EncodeCaller = zapcore.ShortCallerEncoder
	}

	if logFormat == "json" {
		encoder = zapcore.NewJSONEncoder(config)
	} else {
		encoder = zapcore.NewConsoleEncoder(config)
	}

	// Set log level
	var level zapcore.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	// Initialize the appropriate log writer
	initializeLogWriter(logWriter, environment)
}

// initializeLogWriter sets up the log writer based on configuration
func initializeLogWriter(writerType, environment string) {
	switch writerType {
	case "loki":
		// Get Loki configuration from environment
		lokiEndpoint := getEnvOrDefault("LOKI_ENDPOINT", "http://loki:3100")

		// Create labels for this instance
		labels := map[string]string{
			"service":     getEnvOrDefault("SERVICE_NAME", "app"),
			"environment": environment,
			"node_id":     getEnvOrDefault("NODE_ID", "unknown"),
			"cluster_id":  getEnvOrDefault("CLUSTER_ID", "unknown"),
		}

		writer = logwriter.NewLokiWriter(lokiEndpoint, labels)

	case "opensearch":
		// Get OpenSearch configuration from environment
		opensearchEndpoint := getEnvOrDefault("OPENSEARCH_ENDPOINT", "http://opensearch:9200")
		indexName := getEnvOrDefault("OPENSEARCH_INDEX", "app-logs")

		writer = logwriter.NewOpenSearchWriter(opensearchEndpoint, indexName)

	default:
		// Default to local writer
		writer = logwriter.NewLocalWriter()
	}
}

// getEnvOrDefault gets environment variable or returns default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getCaller(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	parts := strings.Split(file, "/")
	if len(parts) > 2 {
		file = strings.Join(parts[len(parts)-2:], "/")
	}
	return fmt.Sprintf("%s:%d", file, line)
}

func LogInfo(_ request.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	entry := logwriter.LogEntry{
		Timestamp: time.Now(),
		Level:     logwriter.InfoLevel,
		Message:   msg,
		Caller:    getCaller(2),
	}
	writer.Write(entry)
	log.Info(msg)
}

func LogInfoWithContext(ctx request.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	entry := logwriter.LogEntry{
		Timestamp: time.Now(),
		Level:     logwriter.InfoLevel,
		Message:   msg,
		Caller:    getCaller(2),
		RequestID: ctx.XID().String(),
		TraceID:   ctx.TraceID(),
	}

	if userInfo := ctx.GetUserInfo(); userInfo != nil {
		entry.UserID = userInfo.GetID().String()
		entry.UserEmail = userInfo.GetEmail()
	}

	writer.Write(entry)
	log.Info(msg,
		zap.String("requestID", entry.RequestID),
		zap.String("traceID", entry.TraceID),
		zap.String("userID", entry.UserID),
	)
}

func LogError(ctx request.Context, err error) {
	if err == nil {
		return
	}

	entry := logwriter.LogEntry{
		Timestamp: time.Now(),
		Level:     logwriter.ErrorLevel,
		Message:   "Error occurred",
		Error:     err.Error(),
		Caller:    getCaller(2),
		RequestID: ctx.XID().String(),
		TraceID:   ctx.TraceID(),
	}

	if userInfo := ctx.GetUserInfo(); userInfo != nil {
		entry.UserID = userInfo.GetID().String()
		entry.UserEmail = userInfo.GetEmail()
	}

	writer.Write(entry)
	log.Error("Error occurred", zap.Error(err))
}

func LogEnter(ctx request.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf("→ ENTER: "+format, args...)
	entry := logwriter.LogEntry{
		Timestamp: time.Now(),
		Level:     logwriter.DebugLevel,
		Message:   msg,
		Caller:    getCaller(2),
		RequestID: ctx.XID().String(),
		TraceID:   ctx.TraceID(),
	}

	if userInfo := ctx.GetUserInfo(); userInfo != nil {
		entry.UserID = userInfo.GetID().String()
		entry.UserEmail = userInfo.GetEmail()
	}

	writer.Write(entry)
	log.Debug(msg)
}

func LogExit(ctx request.Context, startTime time.Time) {
	duration := time.Since(startTime)
	entry := logwriter.LogEntry{
		Timestamp: time.Now(),
		Level:     logwriter.DebugLevel,
		Message:   "← EXIT",
		Caller:    getCaller(2),
		RequestID: ctx.XID().String(),
		TraceID:   ctx.TraceID(),
		Duration:  &duration,
	}

	if userInfo := ctx.GetUserInfo(); userInfo != nil {
		entry.UserID = userInfo.GetID().String()
		entry.UserEmail = userInfo.GetEmail()
	}

	writer.Write(entry)
	log.Debug("← EXIT", zap.Duration("duration", duration))
}

func SetWriter(w logwriter.LogWriter) {
	writer = w
}

func Sync() {
	_ = log.Sync()
	if writer != nil {
		_ = writer.Flush()
	}
}
