package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/s-588/mesh-network/internal/routing"
	"github.com/s-588/mesh-network/internal/socket"
)

// StartIPCServer function starts node's IPC server to communicate with CLI.
func StartIPCServer(t *socket.Socket) {
	mux := http.NewServeMux()
	setupHandlers(mux, t)

	slog.Info("IPC server started", "port", IPCPort)
	server := &http.Server{
		Addr:         "127.0.0.1" + IPCPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		slog.Error("IPC server crashed", "error", err)
	}
}

func setupHandlers(mux *http.ServeMux, t *socket.Socket) {
	handleSend(mux, t)
	handleMessages(mux, t)
	handleRREQ(mux, t)
	handleRoutes(mux)
	handleNeighbours(mux)
}

func handleNeighbours(mux *http.ServeMux) {
	mux.HandleFunc("/neighbours", func(w http.ResponseWriter, _ *http.Request) {
		neighMap := routing.NeighboursTable.Snapshot()
		list := make([]NeighDTO, 0, len(neighMap))
		for _, v := range neighMap {
			list = append(list, parseNeighDTO(v))
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(list)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "can't write response for node: %v\n", err)
		}
	})
}

func handleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/routes", func(w http.ResponseWriter, _ *http.Request) {
		routesMap := routing.RoutesTable.Snapshot()
		list := make([]RouteDTO, 0, len(routesMap))
		for _, v := range routesMap {
			list = append(list, parseRouteDTO(v))
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(list)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "can't write response for node: %v\n", err)
		}
	})
}

func handleRREQ(mux *http.ServeMux, t *socket.Socket) {
	mux.HandleFunc("/rreq", func(w http.ResponseWriter, r *http.Request) {
		dstStr := r.URL.Query().Get("dst")
		dst, err := strconv.ParseUint(dstStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid destination ID", http.StatusBadRequest)
			return
		}

		t.SendRREQ(dst)
		_, _ = fmt.Fprintf(w, "Route request sent for node %d\n", dst)
	})
}

func handleMessages(mux *http.ServeMux, t *socket.Socket) {
	mux.HandleFunc("/messages", func(w http.ResponseWriter, _ *http.Request) {
		msgs := t.GetMessages()
		data, _ := json.Marshal(msgs)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(data)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "can't write response for node: %v\n", err)
		}
	})
}

func handleSend(mux *http.ServeMux, t *socket.Socket) {
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		dstStr := r.URL.Query().Get("dst")
		msg := r.URL.Query().Get("msg")

		dst, err := strconv.ParseUint(dstStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid destination ID", http.StatusBadRequest)
			return
		}

		t.SendData(dst, []byte(msg))
		_, _ = fmt.Fprintf(w, "Message added to queue for node %d\n", dst)
	})
}
