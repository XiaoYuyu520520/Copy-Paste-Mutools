package loglib

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	fileLogger *log.Logger
	logFile    io.Closer
)

func Init(logDir string) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(logDir, fmt.Sprintf("netspeed_%s.log", time.Now().Format("20060102_150405")))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	logFile = f
	fileLogger = log.New(f, "", log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	log.Printf("[LOG] Log file: %s", path)
	return nil
}

func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

func Debug(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[DEBUG] %s", msg)
}

func Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[INFO] %s", msg)
}

func Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[WARN] %s", msg)
}

func Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[ERROR] %s", msg)
}
