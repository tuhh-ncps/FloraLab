package utils

import (
	"fmt"
	"log"
	"os"
)

// Logger is a simple logging utility
type Logger struct {
	verbose bool
}

// NewLogger creates a new logger instance
func NewLogger(verbose bool) *Logger {
	return &Logger{verbose: verbose}
}

// Info prints an info message
func (l *Logger) Info(format string, args ...interface{}) {
	fmt.Printf("ℹ "+format+"\n", args...)
}

// Success prints a success message
func (l *Logger) Success(format string, args ...interface{}) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// Error prints an error message
func (l *Logger) Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
}

// Warning prints a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	fmt.Printf("⚠ "+format+"\n", args...)
}

// Debug prints a debug message if verbose mode is enabled
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.verbose {
		fmt.Printf("🔍 "+format+"\n", args...)
	}
}

// Fatal prints an error message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.Error(format, args...)
	os.Exit(1)
}

// DefaultLogger is the default logger instance
var DefaultLogger = NewLogger(false)

// Helper functions for quick access
func Info(format string, args ...interface{}) {
	DefaultLogger.Info(format, args...)
}

func Success(format string, args ...interface{}) {
	DefaultLogger.Success(format, args...)
}

func Error(format string, args ...interface{}) {
	DefaultLogger.Error(format, args...)
}

func Warning(format string, args ...interface{}) {
	DefaultLogger.Warning(format, args...)
}

func Debug(format string, args ...interface{}) {
	DefaultLogger.Debug(format, args...)
}

func Fatal(format string, args ...interface{}) {
	log.Fatalf("✗ "+format, args...)
}
