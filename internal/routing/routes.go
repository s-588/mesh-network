package routing

import (
	"net/netip"
	"sync"
	"time"
)

// Table is a generic type for creating thread-safe NeighboursTable and RoutesTable
type Table[T any] struct{
	rw sync.RWMutex
	m map[uint64]T
}

func (t *Table[T]) Get(id uint64) T{
	t.rw.RLock()
	defer t.rw.RUnlock()
	return t.m[id]
}

func (t *Table[T]) Put(id uint64, v T){
	t.rw.RLock()
	defer t.rw.RUnlock()
	t.m[id] = v
}

var (
	NeighboursTable Table[NeighboursEntry]
	RoutesTable Table[RouteEntry]
)

type NeighboursEntry struct{
	ID uint64
	Addr netip.AddrPort
	LastSeen time.Time
}

type RouteEntry struct{
	DstID uint64 // ID of end node.
	DstSeq uint32 // Number of freshest route.
	HopCount uint8 // Count of hops till the destination.
	NextHopID uint64 // ID of the next node.
	NextHopAddr netip.AddrPort // Port to where send messages of the next node.
	Lifetime time.Time // Absolute expiration time after which the route is considered invalid and must be removed
	LastUpdate time.Time // Time when route was updated last time.
	Precursors map[uint64]struct{} // IDs of all nodes who should recieve notification when something happens with route.
	Interface string // Linux interface name for sending
}