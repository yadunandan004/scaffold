package logwriter

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[37m"
)

type LocalWriter struct {
	mu sync.Mutex
}

func NewLocalWriter() *LocalWriter {
	return &LocalWriter{}
}

func (w *LocalWriter) Write(entry LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	levelColor := w.getLevelColor(entry.Level)
	timeStr := entry.Timestamp.Format("15:04:05.000")

	fmt.Fprintf(os.Stdout, "%s%s%s %s%-5s%s ",
		colorGray, timeStr, colorReset,
		levelColor, entry.Level, colorReset)

	if entry.Caller != "" {
		fmt.Fprintf(os.Stdout, "%s%s%s ", colorCyan, entry.Caller, colorReset)
	}

	fmt.Fprintf(os.Stdout, "%s", entry.Message)

	var fields []string
	if entry.RequestID != "" {
		fields = append(fields, fmt.Sprintf("request_id=%s%s%s", colorBlue, entry.RequestID, colorReset))
	}
	if entry.TraceID != "" {
		fields = append(fields, fmt.Sprintf("trace_id=%s%s%s", colorBlue, entry.TraceID, colorReset))
	}
	if entry.UserID != "" {
		fields = append(fields, fmt.Sprintf("user_id=%s%s%s", colorPurple, entry.UserID, colorReset))
	}
	if entry.UserEmail != "" {
		fields = append(fields, fmt.Sprintf("user_email=%s%s%s", colorPurple, entry.UserEmail, colorReset))
	}
	if entry.Duration != nil {
		fields = append(fields, fmt.Sprintf("duration=%s%v%s", colorGreen, *entry.Duration, colorReset))
	}
	if entry.Error != "" {
		fields = append(fields, fmt.Sprintf("error=%s%s%s", colorRed, entry.Error, colorReset))
	}

	for k, v := range entry.Fields {
		fields = append(fields, fmt.Sprintf("%s=%v", k, v))
	}

	if len(fields) > 0 {
		fmt.Fprintf(os.Stdout, " %s{%s %s}%s",
			colorGray, strings.Join(fields, " "), colorGray, colorReset)
	}

	fmt.Fprintln(os.Stdout)
	return nil
}

func (w *LocalWriter) Flush() error {
	return nil
}

func (w *LocalWriter) getLevelColor(level LogLevel) string {
	switch level {
	case DebugLevel:
		return colorGray
	case InfoLevel:
		return colorGreen
	case WarnLevel:
		return colorYellow
	case ErrorLevel:
		return colorRed
	default:
		return colorReset
	}
}
