package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// ANSI 颜色码：仅终端输出时使用（TTY 检测），重定向/日志文件自动无色
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

// useColor 是否启用彩色：标准输出为终端且未设置 NO_COLOR
var useColor = term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == ""

// levelColor 返回级别对应前景色（无色模式返回空串）
func levelColor(l slog.Level) string {
	if !useColor {
		return ""
	}
	switch {
	case l >= slog.LevelError:
		return colorRed
	case l >= slog.LevelWarn:
		return colorYellow
	case l >= slog.LevelInfo:
		return colorGreen
	default:
		return colorBlue // debug/trace
	}
}

// prettyHandler 人类友好的 slog Handler：输出 `[2026-08-28 20:18:00] INFO message [key=value]`，
// LEVEL 按级别着色（ERROR 红 / WARN 黄 / INFO 绿 / DEBUG 蓝），对齐原 logrus 前缀格式的阅读习惯。
type prettyHandler struct {
	level *slog.LevelVar
	out   io.Writer
	attrs []slog.Attr
	// 指针而非值：WithAttrs 会复制 handler，值类型 sync.Mutex 会触发 go vet copylocks
	mu *sync.Mutex
}

func newPrettyHandler(out io.Writer, level *slog.LevelVar) *prettyHandler {
	return &prettyHandler{level: level, out: out, mu: &sync.Mutex{}}
}

func (h *prettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer
	lvlStr := strings.ToUpper(r.Level.String())
	// 固定宽度 5 对齐（% -5s），着色码不计入宽度
	if len(lvlStr) < 5 {
		lvlStr += strings.Repeat(" ", 5-len(lvlStr))
	}
	colored := lvlStr
	if c := levelColor(r.Level); c != "" {
		colored = c + lvlStr + colorReset
	}
	fmt.Fprintf(&buf, "[%s] %s %s", r.Time.Format("2006-01-02 15:04:05"), colored, r.Message)
	if len(h.attrs) > 0 {
		for _, a := range h.attrs {
			fmt.Fprintf(&buf, " %s=%v", a.Key, a.Value.Any())
		}
	}
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&buf, " %s=%v", a.Key, a.Value.Any())
		return true
	})
	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf.Bytes())
	return err
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cp := *h
	cp.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &cp
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	// 项目日志无分组需求，原样返回
	return h
}
