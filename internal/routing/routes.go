package routing

import (
	"fmt"
	"net/netip"
	"sort"
	"strconv"
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

// Helper to format AddrPort as string
func addrPortString(ap netip.AddrPort) string {
	return ap.String()
}

// Helper to format time as relative string
func timeString(tm time.Time) string {
	if tm.IsZero() {
		return "never"
	}
	return tm.Format(time.DateTime)
}

func (t *Table[T]) String() string {
	t.rw.RLock()
	defer t.rw.RUnlock()

	if len(t.m) == 0 {
		return "<empty>"
	}

	keys := make([]uint64, 0, len(t.m))
	for k := range t.m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	var sb strings.Builder
	sb.Grow(512 * len(keys))

	for _, id := range keys {
		entry := t.m[id]
		switch v := any(&entry).(type) {
		case *NeighboursEntry:
			sb.WriteString(fmt.Sprintf("ID: %d | Addr: %s | LastSeen: %s\n",
				v.ID, addrPortString(v.Addr), timeString(v.LastSeen)))
		case *RouteEntry:
			precursors := make([]string, 0, len(v.Precursors))
			for p := range v.Precursors {
				precursors = append(precursors, strconv.FormatUint(p, 10))
			}
			sort.Strings(precursors)
			precStr := strings.Join(precursors, ", ")
			if precStr == "" {
				precStr = "none"
			}
			sb.WriteString(fmt.Sprintf("DstID: %d | Seq: %d | Hops: %d | NextHop: %d (%s) | Lifetime: %s | LastUpdate: %s | Precursors: [%s] | Iface: %s\n",
				v.DstID, v.DstSeq, v.HopCount, v.NextHopID, addrPortString(v.NextHopAddr),
				timeString(v.Lifetime), timeString(v.LastUpdate), precStr, v.Interface))
		default:
			// Fallback for unknown types or future additions
			sb.WriteString(fmt.Sprintf("ID: %d | %+v\n", id, entry))
		}
	}

	return sb.String()
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
