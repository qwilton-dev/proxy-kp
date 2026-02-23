package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	zapLogger *zap.Logger
	sugar     *zap.SugaredLogger
}

func New(level string, format string) (*Logger, error) {
	var config zap.Config

	if format == "console" {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}

	if level != "" {
		var lvl zapcore.Level
		if err := lvl.UnmarshalText([]byte(level)); err != nil {
			return nil, fmt.Errorf("invalid log level: %s", level)
		}
		config.Level = zap.NewAtomicLevelAt(lvl)
	}

	zapLogger, err := config.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &Logger{
		zapLogger: zapLogger,
		sugar:     zapLogger.Sugar(),
	}, nil
}

func (l *Logger) Sync() error {
	return l.zapLogger.Sync()
}

func (l *Logger) With(fields ...zap.Field) *Logger {
	newZapLogger := l.zapLogger.With(fields...)
	return &Logger{
		zapLogger: newZapLogger,
		sugar:     newZapLogger.Sugar(),
	}
}

func (l *Logger) Debug(args ...interface{}) {
	l.sugar.Debug(args...)
}

func (l *Logger) Debugf(template string, args ...interface{}) {
	l.sugar.Debugf(template, args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.sugar.Info(args...)
}

func (l *Logger) Infof(template string, args ...interface{}) {
	l.sugar.Infof(template, args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.sugar.Warn(args...)
}

func (l *Logger) Warnf(template string, args ...interface{}) {
	l.sugar.Warnf(template, args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.sugar.Error(args...)
}

func (l *Logger) Errorf(template string, args ...interface{}) {
	l.sugar.Errorf(template, args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.sugar.Fatal(args...)
}

func (l *Logger) Fatalf(template string, args ...interface{}) {
	l.sugar.Fatalf(template, args...)
}

func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.With(zap.String("request_id", requestID))
}

func (l *Logger) WithBackend(backend string) *Logger {
	return l.With(zap.String("backend", backend))
}

func (l *Logger) Zap() *zap.Logger {
	return l.zapLogger
}
