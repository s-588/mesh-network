package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func useServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	prev := ipcBase
	ipcBase = *u
	t.Cleanup(func() { ipcBase = prev })
}

func TestCheckMsg(t *testing.T) {
	t.Parallel()
	if err := CheckMsg("hello"); err != nil {
		t.Fatal(err)
	}
	if err := CheckMsg("   "); err == nil {
		t.Fatal("expected empty message error")
	}
	if err := CheckMsg(strings.Repeat("a", MaxMsgLen+1)); err == nil {
		t.Fatal("expected length error")
	}
	if err := CheckMsg(string([]byte{0xff, 0xfe})); err == nil {
		t.Fatal("expected utf-8 error")
	}
}

func TestSendMsgRejectsEmptyWithoutRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("server should not be called")
	}))
	defer srv.Close()
	useServer(t, srv)

	_, err := SendMsg(context.Background(), 1, " ")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestClientRoundTrip(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/neighbours", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":7,"addr":"10.0.0.2:9000","last_seen":"never","iface":"eth0"}]`))
	})
	mux.HandleFunc("/routes", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"dst_id":9,"next_hop_id":7,"next_hop":"10.0.0.2:9000","hops":2,"seq":4,"iface":"eth0"}]`))
	})
	mux.HandleFunc("/messages", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`["from 1: hi"]`))
	})
	mux.HandleFunc("/config", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":3,"port":9000,"interfaces":["eth0"],"ttl":32,"lifetime":60,"hello_interval":5,"log_level":"INFO","log_file":"","is_daemon":true}`))
	})
	mux.HandleFunc("/rreq", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("dst") != "9" {
			http.Error(w, "bad dst", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RREQResultDTO{
			RouteKnown: false,
			RREQSent:   true,
			Dst:        9,
			Message:    "RREQ broadcast for node 9.",
		})
	})
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("msg") != "hello world" || r.URL.Query().Get("dst") != "9" {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SendResultDTO{
			Sent:     false,
			Queued:   true,
			QueueLen: 1,
			Dst:      9,
			Message:  "Route to node 9 unknown. Message queued (queue length 1). RREQ sent.",
		})
	})
	mux.HandleFunc("/kill", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "use POST /kill to stop the node", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte("Shutting down node 3\n"))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	useServer(t, srv)

	ctx := context.Background()
	if !IsNodeRunning(ctx) {
		t.Fatal("expected node to look running")
	}

	neigh, err := GetNeighbours(ctx)
	if err != nil || len(neigh) != 1 || neigh[0].ID != 7 || neigh[0].Iface != "eth0" {
		t.Fatalf("neighbours: %+v %v", neigh, err)
	}

	routes, err := GetRoutes(ctx)
	if err != nil || len(routes) != 1 || routes[0].DstID != 9 || routes[0].Hops != 2 || routes[0].NextHopID != 7 {
		t.Fatalf("routes: %+v %v", routes, err)
	}

	msgs, err := GetMessages(ctx)
	if err != nil || len(msgs) != 1 || msgs[0] != "from 1: hi" {
		t.Fatalf("messages: %+v %v", msgs, err)
	}

	cfg, err := GetConfig(ctx)
	if err != nil || cfg.ID != 3 || cfg.Port != 9000 || !cfg.IsDaemon || cfg.HelloInterval != 5 {
		t.Fatalf("config: %+v %v", cfg, err)
	}

	rreq, err := SendRREQ(ctx, 9)
	if err != nil || !strings.Contains(rreq.Message, "node 9") {
		t.Fatalf("rreq: %v %v", rreq, err)
	}

	sent, err := SendMsg(ctx, 9, "hello world")
	if err != nil || !strings.Contains(sent.Message, "node 9") {
		t.Fatalf("send: %v %v", sent, err)
	}

	killed, err := KillNode(ctx)
	if err != nil || !strings.Contains(killed, "Shutting down") {
		t.Fatalf("kill: %q %v", killed, err)
	}
}

func TestDoIPCErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid destination ID", http.StatusBadRequest)
	}))
	defer srv.Close()
	useServer(t, srv)

	_, err := SendRREQ(context.Background(), 1)
	if err == nil || err.Error() != "invalid destination ID" {
		t.Fatalf("got %v", err)
	}
}

func TestBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()
	useServer(t, srv)

	if _, err := GetMessages(context.Background()); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestNodeDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()
	useServer(t, srv)

	if IsNodeRunning(context.Background()) {
		t.Fatal("expected down")
	}
	_, err := GetConfig(context.Background())
	if !errors.Is(err, ErrNodeUnattainable) {
		t.Fatalf("got %v", err)
	}
}
