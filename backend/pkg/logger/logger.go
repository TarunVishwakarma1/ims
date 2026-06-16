package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// New builds a zap.Logger that writes to BOTH the console (stdout) and a
// rotating log file. The file path is taken from LOG_FILE_PATH env var,
// defaulting to ./logs/backend.log. Rotation is handled by lumberjack —
// 50MB per file, 7 day retention, gzip compressed old files.
//
// Env controls verbosity + encoding:
//   - env == "production": JSON, info level
//   - else:                console (human-readable), debug level
func New(env string) (*zap.Logger, error) {
	logPath := os.Getenv("LOG_FILE_PATH")
	if logPath == "" {
		logPath = "logs/backend.log"
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, err
	}

	level := zapcore.DebugLevel
	if env == "production" {
		level = zapcore.InfoLevel
	}

	// Console encoder — colored, human-readable
	consoleEncCfg := zap.NewDevelopmentEncoderConfig()
	consoleEncCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEnc := zapcore.NewConsoleEncoder(consoleEncCfg)

	// File encoder — JSON for grep/jq/structured search
	fileEncCfg := zap.NewProductionEncoderConfig()
	fileEncCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEnc := zapcore.NewJSONEncoder(fileEncCfg)

	// Rotating file writer
	fileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    50, // megabytes per file
		MaxBackups: 10,
		MaxAge:     7, // days
		Compress:   true,
	})

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEnc, zapcore.AddSync(os.Stdout), level),
		zapcore.NewCore(fileEnc, fileWriter, level),
	)

	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}
