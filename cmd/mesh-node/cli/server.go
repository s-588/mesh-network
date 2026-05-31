package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/s-588/mesh-network/internal/routing"
	"github.com/s-588/mesh-network/internal/socket"
)

func StartIPCServer(t *socket.Socket) {
	mux := http.NewServeMux()

	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		dstStr := r.URL.Query().Get("dst")
		msg := r.URL.Query().Get("msg")

		dst, err := strconv.ParseUint(dstStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid destination ID", http.StatusBadRequest)
			return
		}

		t.SendData(dst, []byte(msg))
		fmt.Fprintf(w, "Message added to queue for node %d\n", dst)
	})

	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		msgs := t.GetMessages()
		data, _ := json.Marshal(msgs)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	mux.HandleFunc("/rreq", func(w http.ResponseWriter, r *http.Request) {
		dstStr := r.URL.Query().Get("dst")
		dst, err := strconv.ParseUint(dstStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid destination ID", http.StatusBadRequest)
			return
		}

		t.SendRREQ(dst)
		fmt.Fprintf(w, "Запрос RREQ отправлен для поиска узла %d\n", dst)
	})

	// Команда получения таблицы маршрутов
	mux.HandleFunc("/routes", func(w http.ResponseWriter, r *http.Request) {
		routesMap := routing.RoutesTable.Snapshot()
		var list []RouteDTO
		for _, v := range routesMap {
			list = append(list, RouteDTO{
				DstID:   v.DstID,
				NextHop: v.NextHopAddr.String(),
				Hops:    v.HopCount,
				Seq:     v.DstSeq,
				Iface:   v.Interface,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})

	// Команда получения таблицы соседей
	mux.HandleFunc("/neighbours", func(w http.ResponseWriter, r *http.Request) {
		neighMap := routing.NeighboursTable.Snapshot()
		var list []NeighDTO
		for _, v := range neighMap {
			list = append(list, NeighDTO{
				ID:       v.ID,
				Addr:     v.Addr.String(),
				LastSeen: time.Since(v.LastSeen).Round(time.Second).String() + " ago",
				Iface:    v.Interface,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})

	slog.Info("IPC server started", "port", IPC_PORT)
	if err := http.ListenAndServe("127.0.0.1"+IPC_PORT, mux); err != nil {
		slog.Error("IPC server crashed", "error", err)
	}
}
