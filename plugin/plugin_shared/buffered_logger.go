package plugin_shared

import (
	"io"
	"log"
	"time"

	"github.com/hashicorp/go-hclog"
)

// StartupLog represents a log entry captured during plugin initialization.
// These logs are buffered to avoid interfering with the RPC handshake process.
type StartupLog struct {
	Level     string                 `json:"level"`     // Log level: "TRACE", "DEBUG", "INFO", "WARN"
	Message   string                 `json:"message"`   // Log message
	Timestamp time.Time              `json:"timestamp"` // When the log was created
	Fields    map[string]interface{} `json:"fields"`    // Additional structured fields
}

// BufferedLogger wraps an hclog.Logger and buffers Warn/Info messages during plugin startup.
// This prevents log output from interfering with the go-plugin RPC handshake.
//
// Usage pattern:
//  1. Create BufferedLogger during plugin initialization
//  2. Use it for all logging during SetEnvironment()
//  3. After RPC handshake, host calls GetStartupLogs() to retrieve buffered messages
//  4. Call StartNormalLogging() to switch to direct logging mode
//
// In debug mode (local testing), buffering is disabled and logs go directly to output.
type BufferedLogger struct {
	realLogger  hclog.Logger
	buffer      []StartupLog
	isBuffering bool
}

// NewBufferedLogger creates a new BufferedLogger that wraps the given logger.
// If debugMode is true, buffering is disabled and logs go directly to output.
func NewBufferedLogger(logger hclog.Logger, debugMode bool) *BufferedLogger {
	return &BufferedLogger{
		realLogger:  logger,
		buffer:      make([]StartupLog, 0),
		isBuffering: !debugMode, // Only buffer in non-debug mode
	}
}

// Trace logs a trace-level message.
// Trace messages are never buffered and always go directly to the underlying logger.
func (bl *BufferedLogger) Trace(msg string, args ...interface{}) {
	// Trace is too verbose for startup logs, always pass through
	bl.realLogger.Trace(msg, args...)
}

// Debug logs a debug-level message.
// Debug messages are never buffered and always go directly to the underlying logger.
func (bl *BufferedLogger) Debug(msg string, args ...interface{}) {
	// Debug logs always pass through immediately
	bl.realLogger.Debug(msg, args...)
}

// Info logs an info-level message.
// During buffering mode (plugin startup), these are captured and not output.
// After StartNormalLogging() is called, they go directly to the underlying logger.
func (bl *BufferedLogger) Info(msg string, args ...interface{}) {
	if bl.isBuffering {
		bl.buffer = append(bl.buffer, StartupLog{
			Level:     "INFO",
			Message:   msg,
			Timestamp: time.Now(),
			Fields:    argsToMap(args),
		})
	} else {
		bl.realLogger.Info(msg, args...)
	}
}

// Warn logs a warning-level message.
// During buffering mode (plugin startup), these are captured and not output.
// After StartNormalLogging() is called, they go directly to the underlying logger.
func (bl *BufferedLogger) Warn(msg string, args ...interface{}) {
	if bl.isBuffering {
		bl.buffer = append(bl.buffer, StartupLog{
			Level:     "WARN",
			Message:   msg,
			Timestamp: time.Now(),
			Fields:    argsToMap(args),
		})
	} else {
		bl.realLogger.Warn(msg, args...)
	}
}

// Error logs an error-level message.
// Error messages are never buffered and always go directly to the underlying logger.
func (bl *BufferedLogger) Error(msg string, args ...interface{}) {
	// Errors are critical and should always be visible immediately
	bl.realLogger.Error(msg, args...)
}

// IsTrace returns true if the logger is configured to emit trace-level logs.
func (bl *BufferedLogger) IsTrace() bool {
	return bl.realLogger.IsTrace()
}

// IsDebug returns true if the logger is configured to emit debug-level logs.
func (bl *BufferedLogger) IsDebug() bool {
	return bl.realLogger.IsDebug()
}

// IsInfo returns true if the logger is configured to emit info-level logs.
func (bl *BufferedLogger) IsInfo() bool {
	return bl.realLogger.IsInfo()
}

// IsWarn returns true if the logger is configured to emit warn-level logs.
func (bl *BufferedLogger) IsWarn() bool {
	return bl.realLogger.IsWarn()
}

// IsError returns true if the logger is configured to emit error-level logs.
func (bl *BufferedLogger) IsError() bool {
	return bl.realLogger.IsError()
}

// SetLevel sets the logging level for the underlying logger.
func (bl *BufferedLogger) SetLevel(level hclog.Level) {
	bl.realLogger.SetLevel(level)
}

// GetLevel returns the current logging level.
func (bl *BufferedLogger) GetLevel() hclog.Level {
	return bl.realLogger.GetLevel()
}

// With creates a new logger with the given key-value pairs added to all log output.
func (bl *BufferedLogger) With(args ...interface{}) hclog.Logger {
	return &BufferedLogger{
		realLogger:  bl.realLogger.With(args...),
		buffer:      bl.buffer, // Share the same buffer
		isBuffering: bl.isBuffering,
	}
}

// Named creates a new logger with the given name appended to the current logger's name.
func (bl *BufferedLogger) Named(name string) hclog.Logger {
	return &BufferedLogger{
		realLogger:  bl.realLogger.Named(name),
		buffer:      bl.buffer, // Share the same buffer
		isBuffering: bl.isBuffering,
	}
}

// ResetNamed creates a new logger with the given name, discarding the current logger's name.
func (bl *BufferedLogger) ResetNamed(name string) hclog.Logger {
	return &BufferedLogger{
		realLogger:  bl.realLogger.ResetNamed(name),
		buffer:      bl.buffer, // Share the same buffer
		isBuffering: bl.isBuffering,
	}
}

// Log is a generic logging method that logs at the specified level.
// This implements the hclog.Logger interface requirement.
func (bl *BufferedLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Trace:
		bl.Trace(msg, args...)
	case hclog.Debug:
		bl.Debug(msg, args...)
	case hclog.Info:
		bl.Info(msg, args...)
	case hclog.Warn:
		bl.Warn(msg, args...)
	case hclog.Error:
		bl.Error(msg, args...)
	default:
		bl.Info(msg, args...)
	}
}

// ImpliedArgs returns the implied arguments (context fields) set on this logger.
// This implements the hclog.Logger interface requirement.
func (bl *BufferedLogger) ImpliedArgs() []interface{} {
	return bl.realLogger.ImpliedArgs()
}

// Name returns the name of the logger.
// This implements the hclog.Logger interface requirement.
func (bl *BufferedLogger) Name() string {
	return bl.realLogger.Name()
}

// StandardLogger returns a standard library logger that writes to this logger.
// This implements the hclog.Logger interface requirement.
func (bl *BufferedLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return bl.realLogger.StandardLogger(opts)
}

// StandardWriter returns a writer that writes to this logger at the specified level.
// This implements the hclog.Logger interface requirement.
func (bl *BufferedLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return bl.realLogger.StandardWriter(opts)
}

// GetStartupLogs returns all buffered startup logs and clears the buffer.
// This should be called by the RPC host after the plugin handshake completes.
// Subsequent calls will return an empty slice.
func (bl *BufferedLogger) GetStartupLogs() []StartupLog {
	logs := bl.buffer
	bl.buffer = make([]StartupLog, 0) // Clear buffer after retrieval
	return logs
}

// StartNormalLogging disables buffering mode and switches to direct logging.
// Call this after the RPC host has retrieved startup logs via GetStartupLogs().
// All subsequent log calls will go directly to the underlying logger.
func (bl *BufferedLogger) StartNormalLogging() {
	bl.isBuffering = false
	bl.buffer = nil // Free memory
}

// argsToMap converts hclog-style variadic args (key1, val1, key2, val2, ...) into a map.
// If args are malformed (odd number of args), the last key will have a nil value.
func argsToMap(args []interface{}) map[string]interface{} {
	fields := make(map[string]interface{})
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			// Normal case: key-value pair
			if key, ok := args[i].(string); ok {
				fields[key] = args[i+1]
			}
		} else {
			// Odd number of args: last key has no value
			if key, ok := args[i].(string); ok {
				fields[key] = nil
			}
		}
	}
	return fields
}
