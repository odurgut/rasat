package stream

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5/middleware"
)

// ServeHTTP upgrades GET to a JSON WebSocket. Each message is one T.
func (h *Hub[T]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	sub, err := h.Subscribe()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	defer sub.Close()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		h.log.Warn("stream accept", "err", err, "request_id", middleware.GetReqID(r.Context()))
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}()

	h.log.Info("stream connect", "request_id", middleware.GetReqID(r.Context()))
	defer h.log.Info("stream disconnect", "request_id", middleware.GetReqID(r.Context()))

	for {
		select {
		case <-ctx.Done():
			return
		case row, ok := <-sub.C():
			if !ok {
				return
			}
			wctx, wcancel := context.WithTimeout(ctx, h.writeTimeout)
			err := wsjson.Write(wctx, conn, row)
			wcancel()
			if err != nil {
				return
			}
		}
	}
}
