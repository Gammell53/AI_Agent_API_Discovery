package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	// Logger is the global logger instance
	Logger *log.Logger
	// logFile is the current log file
	logFile *os.File
)

// InitLogger initializes the logger with both file and console output
func InitLogger() error {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join("logs", fmt.Sprintf("discovery_%s.log", timestamp))

	file, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// Close existing log file if any
	if logFile != nil {
		logFile.Close()
	}
	logFile = file

	// Create multi-writer for both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, file)

	// Create new logger
	Logger = log.New(multiWriter, "", log.Ldate|log.Ltime|log.Lmicroseconds)

	Logger.Printf("Initialized logging to %s", logPath)
	return nil
}

// CloseLogger closes the log file
func CloseLogger() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}
