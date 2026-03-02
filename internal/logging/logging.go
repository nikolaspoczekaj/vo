package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level represents the severity of a log message.
type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// popupFuncType is an optional callback used to show popups in the UI.
// It is set from main/editor code and is not required for file logging.
type popupFuncType func(text string, level Level)

var (
	mu       sync.Mutex
	logger   *log.Logger
	logFile  *os.File
	popupFn  popupFuncType
	initOnce sync.Once
)

// logFilePath returns the OS-specific path for the log file.
// We use the same base as the config: UserConfigDir/vo/vo.log
func logFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vo", "vo.log"), nil
}

func initLogger() error {
	var err error
	initOnce.Do(func() {
		path, e := logFilePath()
		if e != nil {
			err = e
			return
		}
		dir := filepath.Dir(path)
		if e := os.MkdirAll(dir, 0755); e != nil {
			err = e
			return
		}
		f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if e != nil {
			err = e
			return
		}
		logFile = f
		logger = log.New(f, "", 0)
	})
	return err
}

// SetPopupFunc installs a callback that will be called for log entries
// where popup == true. This is typically wired to Editor.ShowPopup.
func SetPopupFunc(fn popupFuncType) {
	mu.Lock()
	defer mu.Unlock()
	popupFn = fn
}

func logMessage(level Level, msg string, popup bool) {
	if err := initLogger(); err != nil {
		// If logger cannot be initialized, fail silently for now.
		return
	}
	now := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf("%s [%s] %s", now, level.String(), msg)

	mu.Lock()
	if logger != nil {
		_ = logger.Output(2, line)
	}
	fn := popupFn
	mu.Unlock()

	if popup && fn != nil {
		fn(msg, level)
	}
}

// Info logs an informational message. If popup is true and a popup
// callback is registered, a popup is shown in the UI.
func Info(msg string, popup bool) {
	logMessage(LevelInfo, msg, popup)
}

// Warn logs a warning message.
func Warn(msg string, popup bool) {
	logMessage(LevelWarn, msg, popup)
}

// Error logs an error message.
func Error(msg string, popup bool) {
	logMessage(LevelError, msg, popup)
}

