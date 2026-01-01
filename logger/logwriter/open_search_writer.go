package logwriter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type OpenSearchWriter struct {
	client    *http.Client
	endpoint  string
	indexName string
	mu        sync.Mutex
	buffer    []LogEntry
	bufferMax int
}

func NewOpenSearchWriter(endpoint, indexName string) *OpenSearchWriter {
	return &OpenSearchWriter{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		endpoint:  endpoint,
		indexName: indexName,
		buffer:    make([]LogEntry, 0, 100),
		bufferMax: 100,
	}
}

func (w *OpenSearchWriter) Write(entry LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer = append(w.buffer, entry)

	if len(w.buffer) >= w.bufferMax {
		return w.flush()
	}

	return nil
}

func (w *OpenSearchWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flush()
}

func (w *OpenSearchWriter) flush() error {
	if len(w.buffer) == 0 {
		return nil
	}

	var bulkBody bytes.Buffer
	for _, entry := range w.buffer {
		meta := map[string]interface{}{
			"index": map[string]string{
				"_index": w.indexName,
			},
		}
		metaJSON, _ := json.Marshal(meta)
		entryJSON, _ := json.Marshal(entry)

		bulkBody.Write(metaJSON)
		bulkBody.WriteByte('\n')
		bulkBody.Write(entryJSON)
		bulkBody.WriteByte('\n')
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/_bulk", w.endpoint), &bulkBody)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opensearch bulk insert failed with status: %d", resp.StatusCode)
	}

	w.buffer = w.buffer[:0]
	return nil
}
