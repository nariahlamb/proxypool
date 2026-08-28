package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// prettyHandler 人类友好的 slog Handler：输出 `[2026-08-28 20:18:00] INFO message [key=value]`，
// 对齐原 logrus-prefixed-formatter 的阅读习惯（替代 TextHandler 的 time=/level=/msg= 键值形式）。
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
	fmt.Fprintf(&buf, "[%s] %-5s %s", r.Time.Format("2006-01-02 15:04:05"),
		strings.ToUpper(r.Level.String()), r.Message)
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
