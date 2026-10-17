package cli

import (
	"time"

	"github.com/s-588/mesh-network/internal/ipc"
	"github.com/s-588/mesh-network/internal/routing"
)

func parseRouteDTO(r routing.RouteEntry) ipc.RouteDTO {
	return ipc.RouteDTO{
		DstID:     r.DstID,
		NextHopID: r.NextHopID,
		NextHop:   r.NextHopAddr.String(),
		Hops:      int(r.HopCount),
		Seq:       r.DstSeq,
		Iface:     r.Interface,
	}
}

func parseNeighDTO(n routing.NeighboursEntry) ipc.NeighDTO {
	seen := "never"
	if !n.LastSeen.IsZero() {
		seen = n.LastSeen.Format(time.DateTime)
	}
	return ipc.NeighDTO{
		ID:       n.ID,
		Addr:     n.Addr.String(),
		LastSeen: seen,
		Iface:    n.Interface,
	}
}
