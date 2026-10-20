package rest

import (
	"context"
	"log/slog"
)

// WithRequestID returns a logger that adds the ID of the request being
// served to every record written with that request's context. The
// composition root hands it to the use cases, which know nothing of
// requests, so that their audit entries can be matched to the request log.
func WithRequestID(log *slog.Logger) *slog.Logger {
	if log == nil {
		log = slog.Default()
	}
	return slog.New(requestIDHandler{log.Handler()})
}

type requestIDHandler struct {
	slog.Handler
}

func (h requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := requestID(ctx); id != "" {
		r = r.Clone()
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return requestIDHandler{h.Handler.WithAttrs(attrs)}
}

func (h requestIDHandler) WithGroup(name string) slog.Handler {
	return requestIDHandler{h.Handler.WithGroup(name)}
}
