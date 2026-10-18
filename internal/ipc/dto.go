package ipc

// RouteDTO is the JSON DTO for a route entry returned by the IPC server.
type RouteDTO struct {
	DstID     uint64 `json:"dst_id"`
	NextHopID uint64 `json:"next_hop_id"`
	NextHop   string `json:"next_hop"`
	Hops      int    `json:"hops"`
	Seq       uint32 `json:"seq"`
	Iface     string `json:"iface"`
}

// NeighDTO is the JSON DTO for a neighbour entry returned by the IPC server.
type NeighDTO struct {
	ID       uint64 `json:"id"`
	Addr     string `json:"addr"`
	LastSeen string `json:"last_seen"`
	Iface    string `json:"iface"`
}

// ConfigDTO is the JSON DTO for the running node's configuration.
type ConfigDTO struct {
	ID            uint64   `json:"id"`
	Port          uint16   `json:"port"`
	Interfaces    []string `json:"interfaces"`
	TTL           uint8    `json:"ttl"`
	Lifetime      uint32   `json:"lifetime"`
	HelloInterval int      `json:"hello_interval"`
	LogLevel      string   `json:"log_level"`
	LogFile       string   `json:"log_file"`
	IsDaemon      bool     `json:"is_daemon"`
}

// LogEntryDTO mirrors logger.Entry.
type LogEntryDTO struct {
	ID   uint64 `json:"id"`
	Line string `json:"line"`
}

// QueueEntryDTO describes one destination's pending backlog.
type QueueEntryDTO struct {
	DstID          uint64 `json:"dst_id"`
	Count          int    `json:"count"`
	OldestQueuedAt string `json:"oldest_queued_at"`
}

// SendResultDTO is what POST /send returns.
type SendResultDTO struct {
	Sent     bool   `json:"sent"`
	Queued   bool   `json:"queued"`
	QueueLen int    `json:"queue_len,omitempty"`
	Dst      uint64 `json:"dst"`
	Message  string `json:"message"`
}

// RREQResultDTO is what GET /rreq returns.
type RREQResultDTO struct {
	RouteKnown bool   `json:"route_known"`
	RREQSent   bool   `json:"rreq_sent"`
	Dst        uint64 `json:"dst"`
	Message    string `json:"message"`
}
