package webui

import (
	"sync"
	"time"
)

const defaultLogCapacity = 1000

// LogEntry is a single runtime log line for the WebUI.
type LogEntry struct {
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Source  string         `json:"source"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// LogStore holds recent WebUI runtime logs in memory.
type LogStore struct {
	mu       sync.RWMutex
	capacity int
	entries  []LogEntry
}

func NewLogStore(capacity int) *LogStore {
	if capacity <= 0 {
		capacity = defaultLogCapacity
	}
	return &LogStore{capacity: capacity}
}

func (s *LogStore) Append(level, source, message string, fields map[string]any) LogEntry {
	entry := LogEntry{
		Time:    time.Now().UTC(),
		Level:   level,
		Source:  source,
		Message: message,
	}
	if len(fields) > 0 {
		entry.Fields = make(map[string]any, len(fields))
		for k, v := range fields {
			entry.Fields[k] = v
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	if len(s.entries) > s.capacity {
		s.entries = s.entries[len(s.entries)-s.capacity:]
	}
	return entry
}

func (s *LogStore) List(level, source string, limit int) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]LogEntry, 0, len(s.entries))
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if level != "" && e.Level != level {
			continue
		}
		if source != "" && e.Source != source {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (s *LogStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
}

func (s *Server) logInfo(source, message string, fields map[string]any) {
	s.logs.Append("info", source, message, fields)
}

func (s *Server) logWarn(source, message string, fields map[string]any) {
	s.logs.Append("warn", source, message, fields)
}

func (s *Server) logError(source, message string, fields map[string]any) {
	s.logs.Append("error", source, message, fields)
}
