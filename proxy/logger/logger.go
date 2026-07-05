package logger

import (
	"io"
	"context"
	"log/slog"
)

type ColorHandler struct {
	slog.Handler
	w io.Writer
}

func New(w io.Writer, opts *slog.HandlerOptions) *ColorHandler {
	return &ColorHandler{
		Handler: slog.NewTextHandler(w, opts),
		w:       w,
	}
}

func (h *ColorHandler) Handle(ctx context.Context, r slog.Record) error {
	var colorCode string
	switch r.Level {
	case slog.LevelDebug:
		colorCode = "\033[32m" // Green
	case slog.LevelInfo:
		colorCode = ""         // Standard
	case slog.LevelWarn:
		colorCode = "\033[33m" // Yellow
	case slog.LevelError:
		colorCode = "\033[31m" // Red
	}

	if colorCode != "" {
		h.w.Write([]byte(colorCode))
	}
	err := h.Handler.Handle(ctx, r)
	if colorCode != "" {
		h.w.Write([]byte("\033[0m"))
	}
	return err
}

func Shortener(maxLen int) func(groups []string, a slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.MessageKey {
			msgStr := a.Value.String()
			
			if len(msgStr) > maxLen {
				a.Value = slog.StringValue(msgStr[:maxLen-3] + "...")
			}
		}
		return a
	}
}
