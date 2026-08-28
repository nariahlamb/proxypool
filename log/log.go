package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	// 仅用于静默第三方库（mihomo 等）写入 logrus 默认 logger 的日志
	log "github.com/sirupsen/logrus"
)

var (
	level     = INFO
	levelVar  = &slog.LevelVar{}
	// logger 是应用自己的 slog 实例（自定义 pretty handler，人类友好格式）。
	logger = slog.New(newPrettyHandler(os.Stdout, levelVar))
	fileMux = sync.Mutex{}
)

func init() {
	levelVar.Set(slog.LevelInfo)
	// 静默第三方库写入 logrus 默认 logger 的输出（mihomo 内部日志）
	log.SetOutput(io.Discard)
}

// toSlogLevel 映射应用日志级别到 slog 级别（TRACE 归入 DEBUG）
func toSlogLevel(l LogLevel) slog.Level {
	switch l {
	case TRACE, DEBUG:
		return slog.LevelDebug
	case INFO:
		return slog.LevelInfo
	case WARNING:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}

func SetLevel(l LogLevel) {
	level = l
	levelVar.Set(toSlogLevel(l))
}

func Traceln(format string, v ...any) {
	logger.Debug(fmt.Sprintf(format, v...))
}

func Debugln(format string, v ...any) {
	logger.Debug(fmt.Sprintf(format, v...))
}

func Infoln(format string, v ...any) {
	logger.Info(fmt.Sprintf(format, v...))
}

func Warnln(format string, v ...any) {
	logger.Warn(fmt.Sprintf(format, v...))
}

func Errorln(format string, v ...any) {
	logger.Error(fmt.Sprintf(format, v...))
}

func Fileln(l LogLevel, data string) {
	if l >= level {
		if f := initFile(filepath.Join(logDir, logFile)); f != nil {
			fileMux.Lock()
			flog := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: toSlogLevel(l)}))
			flog.Log(context.Background(), toSlogLevel(l), data)
			fileMux.Unlock()
			_ = f.Close()
		}
	}
}
