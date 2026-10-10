// Package logger 构建进程级 zap 日志器，统一时间/级别编码格式。
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"pet-manager/server/internal/config"
)

// New 构建全局日志器。dev 用带颜色的控制台输出，prod 用 JSON 便于采集。
func New(cfg config.LogConfig) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if err := level.Set(cfg.Level); err != nil {
		return nil, err
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder

	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderCfg.ConsoleSeparator = "  "
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}

	core := zapcore.NewCore(encoder, sink(cfg.Output), level)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}

// sink 目前支持 stdout / stderr，落盘交给容器日志采集处理。
func sink(output string) zapcore.WriteSyncer {
	if output == "stderr" {
		return zapcore.Lock(os.Stderr)
	}
	return zapcore.Lock(os.Stdout)
}
