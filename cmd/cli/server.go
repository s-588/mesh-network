package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/s-588/mesh-network/internal/config"
	"github.com/s-588/mesh-network/internal/ipc"
	"github.com/s-588/mesh-network/internal/routing"
	"github.com/s-588/mesh-network/internal/socket"

	nlog "github.com/s-588/mesh-network/internal/logger"
)

// StartIPCServer starts the node's IPC server and blocks until it stops.
// Cancelling ctx, or POST /kill, shuts the server down and returns nil.
// A bind failure is returned so the daemon process exits instead of lingering
// with no CLI interface.
func StartIPCServer(ctx context.Context, t *socket.Socket, cfg config.Config, logBuf *nlog.Buffer) error {
	ctx, stop := context.WithCancel(ctx)
	defer stop()

	mux := http.NewServeMux()
	setupHandlers(mux, t, cfg, logBuf, stop)

	server := &http.Server{
		Addr:         "127.0.0.1" + ipc.IPCPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("IPC server started", "port", ipc.IPCPort)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(shutCtx); err != nil {
			slog.Error("IPC server shutdown", "error", err)
		}
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		slog.Info("IPC server stopped")
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("IPC server crashed", "error", err)
			return fmt.Errorf("IPC server: %w", err)
		}
		return nil
	}
}

func setupHandlers(mux *http.ServeMux, t *socket.Socket, cfg config.Config, logBuf *nlog.Buffer, stop context.CancelFunc) {
	handleSend(mux, t)
	handleMessages(mux, t)
	handleRREQ(mux, t)
	handleRoutes(mux)
	handleNeighbours(mux)
	handleConfig(mux, cfg)
	handleKill(mux, cfg, stop)
	handleLogs(mux, logBuf)
	handleQueue(mux, t)
}

func handleKill(mux *http.ServeMux, cfg config.Config, stop context.CancelFunc) {
	mux.HandleFunc("/kill", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "use POST /kill", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "Shutting down node %d\n", cfg.ID)
		slog.Info("shutdown requested", "id", cfg.ID)

		// Cancel after the response has been flushed. Shutdown waits for the
		// handler to return, so we schedule stop on a dedicated goroutine and
		// give the writer a moment.
		shutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		_ = shutCtx
		go func() {
			defer cancel()
			time.Sleep(200 * time.Millisecond)
			stop()
		}()
	})
}

func handleConfig(mux *http.ServeMux, cfg config.Config) {
	mux.HandleFunc("/config", func(w http.ResponseWriter, _ *http.Request) {
		dto := ipc.ConfigDTO{
			ID:            cfg.ID,
			Port:          cfg.Port,
			Interfaces:    cfg.Interfaces,
			TTL:           cfg.TTL,
			Lifetime:      cfg.Lifetime,
			HelloInterval: cfg.HelloInterval,
			LogLevel:      cfg.Level,
			LogFile:       cfg.LogFile,
			IsDaemon:      cfg.IsDaemon,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(dto); err != nil {
			slog.Error("can't write config response", "error", err)
		}
	})
}

func handleNeighbours(mux *http.ServeMux) {
	mux.HandleFunc("/neighbours", func(w http.ResponseWriter, _ *http.Request) {
		neighMap := routing.NeighboursTable.Snapshot()
		list := make([]ipc.NeighDTO, 0, len(neighMap))
		for _, v := range neighMap {
			list = append(list, parseNeighDTO(v))
		}
		sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(list); err != nil {
			slog.Error("can't write neighbours response", "error", err)
		}
	})
}

func handleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/routes", func(w http.ResponseWriter, _ *http.Request) {
		routesMap := routing.RoutesTable.Snapshot()
		list := make([]ipc.RouteDTO, 0, len(routesMap))
		for _, v := range routesMap {
			list = append(list, parseRouteDTO(v))
		}
		sort.Slice(list, func(i, j int) bool { return list[i].DstID < list[j].DstID })

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(list); err != nil {
			slog.Error("can't write routes response", "error", err)
		}
	})
}

func handleRREQ(mux *http.ServeMux, t *socket.Socket) {
	mux.HandleFunc("/rreq", func(w http.ResponseWriter, r *http.Request) {
		dstStr := r.URL.Query().Get("dst")
		dst, err := strconv.ParseUint(dstStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid destination ID", http.StatusBadRequest)
			return
		}
		_, known := routing.FindRoute(dst)
		dto := ipc.RREQResultDTO{RouteKnown: known, RREQSent: !known, Dst: dst}
		if known {
			dto.Message = fmt.Sprintf("Route to node %d already known, RREQ skipped.", dst)
		} else {
			t.SendRREQ(dst)
			dto.Message = fmt.Sprintf("RREQ broadcast for node %d.", dst)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(dto)
	})
}

func handleMessages(mux *http.ServeMux, t *socket.Socket) {
	mux.HandleFunc("/messages", func(w http.ResponseWriter, _ *http.Request) {
		data, err := json.Marshal(t.GetMessages())
		if err != nil {
			http.Error(w, "can't encode messages", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(data); err != nil {
			slog.Error("can't write messages response", "error", err)
		}
	})
}

func handleSend(mux *http.ServeMux, t *socket.Socket) {
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		dstStr := r.URL.Query().Get("dst")
		msg := r.URL.Query().Get("msg")

		dst, err := strconv.ParseUint(dstStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid destination ID", http.StatusBadRequest)
			return
		}
		if err := ipc.CheckMsg(msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		out := t.SendData(dst, []byte(msg))
		dto := ipc.SendResultDTO{
			Sent: out.Sent, Queued: out.Queued,
			QueueLen: out.QueueLen, Dst: out.Dst,
		}
		switch {
		case out.Sent:
			dto.Message = fmt.Sprintf("Message sent immediately to node %d.", dst)
		case out.Queued:
			dto.Message = fmt.Sprintf(
				"Route to node %d unknown. Message queued (queue length %d). RREQ sent.",
				dst, out.QueueLen)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			dto.Message = "Message could not be sent."
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(dto)
	})
}

func handleLogs(mux *http.ServeMux, buf *nlog.Buffer) {
	if buf == nil {
		return
	}
	mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)
		entries := buf.Since(since)
		dto := make([]ipc.LogEntryDTO, 0, len(entries))
		for _, e := range entries {
			dto = append(dto, ipc.LogEntryDTO{ID: e.ID, Line: e.Line})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(dto); err != nil {
			slog.Error("can't write logs response", "error", err)
		}
	})
}

func handleQueue(mux *http.ServeMux, t *socket.Socket) {
	mux.HandleFunc("/queue", func(w http.ResponseWriter, _ *http.Request) {
		list := t.GetQueue()
		dto := make([]ipc.QueueEntryDTO, 0, len(list))
		for _, q := range list {
			dto = append(dto, ipc.QueueEntryDTO{
				DstID:          q.DstID,
				Count:          q.Count,
				OldestQueuedAt: q.OldestQueuedAt.Format(time.RFC3339),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(dto); err != nil {
			slog.Error("can't write queue response", "error", err)
		}
	})
}
