package logwriter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type LokiWriter struct {
	client    *http.Client
	endpoint  string
	labels    map[string]string
	mu        sync.Mutex
	buffer    []LogEntry
	bufferMax int
}

type LokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

type LokiPayload struct {
	Streams []LokiStream `json:"streams"`
}

func NewLokiWriter(endpoint string, labels map[string]string) *LokiWriter {
	if labels == nil {
		labels = make(map[string]string)
	}

	// Ensure required labels
	if labels["service"] == "" {
		labels["service"] = "furio"
	}

	return &LokiWriter{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		endpoint:  endpoint,
		labels:    labels,
		buffer:    make([]LogEntry, 0, 100),
		bufferMax: 100,
	}
}

func (w *LokiWriter) Write(entry LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer = append(w.buffer, entry)

	if len(w.buffer) >= w.bufferMax {
		return w.flush()
	}

	return nil
}

func (w *LokiWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flush()
}

func (w *LokiWriter) flush() error {
	if len(w.buffer) == 0 {
		return nil
	}

	// Group log entries by their label combination
	streamMap := make(map[string]*LokiStream)

	for _, entry := range w.buffer {
		// Create stream labels for this entry
		streamLabels := w.createStreamLabels(entry)

		// Create a unique key for this label combination
		labelKey := w.createLabelKey(streamLabels)

		// Get or create stream for this label combination
		if streamMap[labelKey] == nil {
			streamMap[labelKey] = &LokiStream{
				Stream: streamLabels,
				Values: make([][]string, 0),
			}
		}

		// Convert entry to log line
		logLine := w.formatLogEntry(entry)
		timestamp := strconv.FormatInt(entry.Timestamp.UnixNano(), 10)

		streamMap[labelKey].Values = append(streamMap[labelKey].Values, []string{timestamp, logLine})
	}

	// Convert to Loki payload
	streams := make([]LokiStream, 0, len(streamMap))
	for _, stream := range streamMap {
		streams = append(streams, *stream)
	}

	payload := LokiPayload{Streams: streams}

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Loki payload: %w", err)
	}

	// Send to Loki
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/loki/api/v1/push", w.endpoint), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create Loki request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send logs to Loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Loki push failed with status: %d", resp.StatusCode)
	}

	// Clear buffer
	w.buffer = w.buffer[:0]
	return nil
}

func (w *LokiWriter) createStreamLabels(entry LogEntry) map[string]string {
	labels := make(map[string]string)

	// Copy base labels
	for k, v := range w.labels {
		labels[k] = v
	}

	// Add entry-specific labels
	labels["level"] = string(entry.Level)

	if entry.RequestID != "" {
		labels["request_id"] = entry.RequestID
	}

	if entry.TraceID != "" {
		labels["trace_id"] = entry.TraceID
	}

	if entry.UserID != "" {
		labels["user_id"] = entry.UserID
	}

	// Add caller as component label (simplified)
	if entry.Caller != "" {
		labels["component"] = w.extractComponent(entry.Caller)
	}

	return labels
}

func (w *LokiWriter) createLabelKey(labels map[string]string) string {
	// Create a consistent key from labels for grouping
	labelBytes, _ := json.Marshal(labels)
	return string(labelBytes)
}

func (w *LokiWriter) formatLogEntry(entry LogEntry) string {
	// Create structured log line as JSON
	logData := map[string]interface{}{
		"timestamp": entry.Timestamp.Format(time.RFC3339Nano),
		"level":     string(entry.Level),
		"message":   entry.Message,
	}

	if entry.Caller != "" {
		logData["caller"] = entry.Caller
	}

	if entry.Error != "" {
		logData["error"] = entry.Error
	}

	if entry.Duration != nil {
		logData["duration"] = entry.Duration.String()
	}

	if entry.UserEmail != "" {
		logData["user_email"] = entry.UserEmail
	}

	// Add custom fields
	if len(entry.Fields) > 0 {
		for k, v := range entry.Fields {
			logData[k] = v
		}
	}

	jsonLine, _ := json.Marshal(logData)
	return string(jsonLine)
}

func (w *LokiWriter) extractComponent(caller string) string {
	// Extract component from caller like "pkg/bus/message.go:123" -> "bus"
	// or "api/health/service.go:45" -> "health"
	parts := bytes.Split([]byte(caller), []byte("/"))
	if len(parts) >= 2 {
		return string(parts[len(parts)-2])
	}
	return "unknown"
}
