package cli

import "github.com/s-588/mesh-network/internal/routing"

// RouteDTO struct contain route's data for transferring with CLI.
type RouteDTO struct {
	DstID   uint64 `json:"dst_id"`
	NextHop string `json:"next_hop"`
	Hops    uint8  `json:"hops"`
	Seq     uint32 `json:"seq"`
	Iface   string `json:"iface"`
}

func parseRouteDTO(r routing.RouteEntry) RouteDTO {
	return RouteDTO{
		DstID:   r.DstID,
		NextHop: r.NextHopAddr.String(),
		Hops:    r.HopCount,
		Seq:     r.DstSeq,
		Iface:   r.Interface,
	}
}

// NeighDTO struct contain neghbours's data for transferring with CLI.
type NeighDTO struct {
	ID       uint64 `json:"id"`
	Addr     string `json:"addr"`
	LastSeen string `json:"last_seen"`
	Iface    string `json:"iface"`
}

func parseNeighDTO(n routing.NeighboursEntry) NeighDTO {
	return NeighDTO{
		ID:       n.ID,
		Addr:     n.Addr.String(),
		LastSeen: n.LastSeen.String(),
		Iface:    n.Interface,
	}
}
