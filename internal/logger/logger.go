// Package logger provides a thread-safe in-memory logger for status messages
package logger

import (
	"sync"
	"time"
)

// Message represents a single log message
type Message struct {
	Timestamp time.Time `json:"timestamp"`
	Text      string    `json:"text"`
	Level     string    `json:"level"` // info, warning, error
}

// Logger manages in-memory log messages
type Logger struct {
	mu       sync.RWMutex
	messages []Message
	maxSize  int
}

// New creates a new logger with specified max message count
func New(maxSize int) *Logger {
	return &Logger{
		messages: make([]Message, 0, maxSize),
		maxSize:  maxSize,
	}
}

// Log adds a new message to the logger
func (l *Logger) Log(level, text string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := Message{
		Timestamp: time.Now(),
		Text:      text,
		Level:     level,
	}

	l.messages = append(l.messages, msg)

	// Keep only the last maxSize messages
	if len(l.messages) > l.maxSize {
		l.messages = l.messages[len(l.messages)-l.maxSize:]
	}
}

// Info logs an info-level message
func (l *Logger) Info(text string) {
	l.Log("info", text)
}

// Warning logs a warning-level message
func (l *Logger) Warning(text string) {
	l.Log("warning", text)
}

// Error logs an error-level message
func (l *Logger) Error(text string) {
	l.Log("error", text)
}

// GetRecent returns the most recent n messages (newest first)
func (l *Logger) GetRecent(n int) []Message {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if n > len(l.messages) {
		n = len(l.messages)
	}

	// Return in reverse order (newest first)
	result := make([]Message, n)
	for i := 0; i < n; i++ {
		result[i] = l.messages[len(l.messages)-1-i]
	}

	return result
}

// GetAll returns all messages (newest first)
func (l *Logger) GetAll() []Message {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return in reverse order (newest first)
	result := make([]Message, len(l.messages))
	for i := 0; i < len(l.messages); i++ {
		result[i] = l.messages[len(l.messages)-1-i]
	}

	return result
}
