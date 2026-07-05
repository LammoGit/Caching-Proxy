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
