package routing

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

// Table is a generic type for creating thread-safe NeighboursTable and RoutesTable
type Table[T any] struct {
	rw sync.RWMutex
	m  map[uint64]T
}

func NewTable[T any]() Table[T] {
	return Table[T]{
		m: make(map[uint64]T),
	}
}

func (t *Table[T]) String() string {
	t.rw.RLock()
	defer t.rw.RUnlock()

	if len(t.m) == 0 {
		return "<empty>"
	}

	ids := make([]uint64, 0, len(t.m))
	for id := range t.m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var b strings.Builder
	b.WriteString("ID\tVALUE\n")
	for _, id := range ids {
		fmt.Fprintf(&b, "%d\t%+v\n", id, t.m[id])
	}
	return b.String()
}

func (t *Table[T]) Get(id uint64) (T, bool) {
	t.rw.RLock()
	defer t.rw.RUnlock()
	v, ok := t.m[id]
	return v, ok
}

func (t *Table[T]) Put(id uint64, v T) {
	t.rw.Lock()
	defer t.rw.Unlock()
	t.m[id] = v
}

func (t *Table[T]) Delete(id uint64) {
	t.rw.Lock()
	defer t.rw.Unlock()
	delete(t.m, id)
}

type NeighboursEntry struct {
	ID       uint64
	Addr     netip.AddrPort
	LastSeen time.Time
}

type RouteEntry struct {
	DstID       uint64              // ID of end node.
	DstSeq      uint32              // Number of freshest route.
	HopCount    uint8               // Count of hops till the destination.
	NextHopID   uint64              // ID of the next node.
	NextHopAddr netip.AddrPort      // Port to where send messages of the next node.
	Lifetime    time.Time           // Absolute expiration time after which the route is considered invalid and must be removed
	LastUpdate  time.Time           // Time when route was updated last time.
	Precursors  map[uint64]struct{} // IDs of all nodes who should recieve notification when something happens with route.
	Interface   string              // Linux interface name for sending
}

var (
	NeighboursTable = NewTable[NeighboursEntry]()
	RoutesTable     = NewTable[RouteEntry]()
)

// FindRoute trying to find route to dstID
// If route exists and it not expired, return entry and true.
func FindRoute(dstID uint64) (RouteEntry, bool) {
	entry, ok := RoutesTable.Get(dstID)
	if !ok {
		return RouteEntry{}, false
	}

	if !entry.Lifetime.IsZero() && time.Now().After(entry.Lifetime) {
		RoutesTable.Delete(dstID)
		return RouteEntry{}, false
	}

	return entry, true
}

// UpdateRoute update or add new route.
// Route will be update if:
// 1. RREP wit higher Sequence Number come.
// 2. Sequence Number same but HopCount smaller.
func UpdateRoute(newEntry RouteEntry) {
	oldEntry, exists := RoutesTable.Get(newEntry.DstID)

	if !exists {
		newEntry.LastUpdate = time.Now()
		RoutesTable.Put(newEntry.DstID, newEntry)
		return
	}

	shouldUpdate := false
	if newEntry.DstSeq > oldEntry.DstSeq {
		shouldUpdate = true
	} else if newEntry.DstSeq == oldEntry.DstSeq && newEntry.HopCount < oldEntry.HopCount {
		shouldUpdate = true
	}

	if shouldUpdate {
		newEntry.LastUpdate = time.Now()
		// keep precursors from old entry
		newEntry.Precursors = oldEntry.Precursors
		RoutesTable.Put(newEntry.DstID, newEntry)
	}
}

// UpdateNeighbour updates neighbour talbe with new HELLO
func UpdateNeighbour(id uint64, addr netip.AddrPort) {
	NeighboursTable.Put(id, NeighboursEntry{
		ID:       id,
		Addr:     addr,
		LastSeen: time.Now(),
	})
}
