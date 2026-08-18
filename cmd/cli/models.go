package cli

// RouteDTO struct contain route's data for transfering with CLI.
type RouteDTO struct {
	DstID   uint64 `json:"dst_id"`
	NextHop string `json:"next_hop"`
	Hops    uint8  `json:"hops"`
	Seq     uint32 `json:"seq"`
	Iface   string `json:"iface"`
}

// NeighDTO struct contain neghbours's data for transfering with CLI.
type NeighDTO struct {
	ID       uint64 `json:"id"`
	Addr     string `json:"addr"`
	LastSeen string `json:"last_seen"`
	Iface    string `json:"iface"`
}
