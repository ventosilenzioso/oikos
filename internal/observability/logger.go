package observability

import (
	"io"
	"strings"

	"github.com/rs/zerolog"
)

type Logger struct{ log zerolog.Logger }

func NewLogger(component string, out io.Writer) *Logger {
	return &Logger{log: zerolog.New(out).With().Timestamp().Str("component", component).Logger()}
}

func (l *Logger) WithServer(serverID string) *Logger {
	if serverID == "" {
		return l
	}
	return &Logger{log: l.log.With().Str("server_id", serverID).Logger()}
}

func (l *Logger) Info(message string, fields ...string) {
	l.write(zerolog.InfoLevel, message, fields...)
}
func (l *Logger) Warn(message string, fields ...string) {
	l.write(zerolog.WarnLevel, message, fields...)
}
func (l *Logger) Error(message string, fields ...string) {
	l.write(zerolog.ErrorLevel, message, fields...)
}

func (l *Logger) write(level zerolog.Level, message string, fields ...string) {
	e := l.log.WithLevel(level)
	for i := 0; i+1 < len(fields); i += 2 {
		key, value := fields[i], fields[i+1]
		if isSecretKey(key) {
			e = e.Str(key, "[REDACTED]")
		} else {
			e = e.Str(key, value)
		}
	}
	e.Msg(message)
}

func RedactSecret(value string) string {
	if value == "" {
		return value
	}
	return "[REDACTED]"
}

func isSecretKey(key string) bool {
	key = strings.ToLower(key)
	for _, sensitive := range []string{"token", "secret", "password", "private_key", "credential", "key"} {
		if key == sensitive || strings.Contains(key, sensitive) {
			return true
		}
	}
	return false
}
