package routing

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestNewTable(t *testing.T) {
	want := make(map[uint64]int)
	get := NewTable[int]()
	if !reflect.DeepEqual(want, get.m) {
		t.Fatalf("TestNewTable error. Want: %v; get:%v", want, get.m)
	}
}

func TestTable_Get(t *testing.T) {
	type args struct {
		id uint64
	}

	table := NewTable[int]()
	table.m[1] = 10

	tests := []struct {
		name   string
		args   args
		want   int
		wantOK bool
	}{
		{
			"Number",
			args{id: 1},
			10,
			true,
		},
		{name: "Not found",
			args: args{
				id: 2,
			},
			want:   0,
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := table.Get(tt.args.id)
			if ok != tt.wantOK {
				t.Errorf("got = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTable_Snapshot(t *testing.T) {
	table := NewTable[int]()
	table.m[1] = 10
	table.m[2] = 20

	snap := table.Snapshot()
	snap[1] = 44
	snap[2] = 98
	snap[3] = 14

	if get, _ := table.m[1]; get == snap[1] {
		t.Errorf("mutating the snaphsot edited original table; got: %d, want: %d", get, 10)
	}

	if _, ok := table.m[3]; ok {
		t.Errorf("mutating the snaphsot leaked into the original table; got: %v, want: %v", ok, false)
	}
}

func TestTable_Put(t *testing.T) {
	table := NewTable[int]()
	table.Put(1, 10)

	if get, ok := table.m[1]; !ok {
		t.Errorf("Put didn't insert value; got: %d, ok: %v, want: %d", get, ok, 10)
	}

	if get, ok := table.Get(3); ok {
		t.Errorf("Put returned value that wasn't inserted; got: %d, ok: %v, want: %v", get, ok, false)
	}
}

func TestTable_Delete(t *testing.T) {
	table := NewTable[int]()
	table.m[1] = 10
	table.Delete(1)
	if get, ok := table.m[1]; ok {
		t.Errorf("value wasn't deleted; got: %d", get)
	}
}

func resetTables(t *testing.T) {
	t.Helper()
	RoutesTable = NewTable[RouteEntry]()
	NeighboursTable = NewTable[NeighboursEntry]()
}

func TestFindRoute(t *testing.T) {
	tests := []struct {
		name   string
		entry  *RouteEntry
		wantOK bool
	}{
		{
			"no route",
			nil,
			false,
		},
		{
			"zero lifetime never expires",
			&RouteEntry{
				DstID:    1,
				HopCount: 2,
			},
			true,
		},
		{
			"lifetime in future is valid",
			&RouteEntry{
				DstID:    1,
				HopCount: 2,
				Lifetime: time.Now().Add(24 * time.Hour),
			},
			true,
		},
		{
			"lifetime is expired",
			&RouteEntry{
				DstID:    1,
				HopCount: 2,
				Lifetime: time.Now().Add(-time.Hour),
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTables(t)
			if tt.entry != nil {
				RoutesTable.Put(1, *tt.entry)
			}
			if _, ok := FindRoute(1); ok != tt.wantOK {
				t.Errorf("FindRoute(1) ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

// FindRoute also need to delete expired entries.
func TestFindRoute_DeleteExpires(t *testing.T) {
	resetTables(t)
	RoutesTable.Put(1, RouteEntry{
		DstID:    1,
		HopCount: 2,
		Lifetime: time.Now().Add(-time.Minute),
	})

	if _, ok := FindRoute(1); ok {
		t.Fatal("route shouldn't been found ")
	}
	if _, ok := FindRoute(1); ok {
		t.Error("expired entry is still there")
	}
}

// TestTable_ConcurrentAccess used for testing of race conditions with -race flag
func TestTable_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	const (
		g       = 50
		opsPerG = 200
		id      = 1
	)
	table := NewTable[int]()

	wg := sync.WaitGroup{}
	wg.Add(g)
	for i := range g {
		go func(i int) {
			defer wg.Done()
			ownID := uint64(i + 1)
			for j := range opsPerG {
				table.Put(ownID, 1)
				table.Put(id, i)
				table.Get(ownID)
				table.Get(id)
				table.Snapshot()
				if j%10 == 0 {
					table.Delete(ownID)
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestUpdateRoute(t *testing.T) {
	tests := []struct {
		name          string
		existingEntry *RouteEntry
		newEntry      RouteEntry
		wantSeq       uint32
		wantHops      uint8
	}{
		{
			name:          "no existing route",
			existingEntry: nil,
			newEntry: RouteEntry{
				DstID:    1,
				DstSeq:   3,
				HopCount: 5,
			},
			wantSeq:  3,
			wantHops: 5,
		},
		{
			name: "higher DstSeq; route should be replaced with new one",
			existingEntry: &RouteEntry{
				DstID:    1,
				DstSeq:   1,
				HopCount: 5,
			},
			newEntry: RouteEntry{
				DstID:    1,
				DstSeq:   3,
				HopCount: 5,
			},
			wantSeq:  3,
			wantHops: 5,
		},
		{
			name: "same DstSeq; less hop count wins",
			existingEntry: &RouteEntry{
				DstID:    1,
				DstSeq:   3,
				HopCount: 4,
			},
			newEntry: RouteEntry{
				DstID:    1,
				DstSeq:   3,
				HopCount: 2,
			},
			wantSeq:  3,
			wantHops: 2,
		},
		{
			name: "same DstSeq; less hop count wins, previous entry kept",
			existingEntry: &RouteEntry{
				DstID:    1,
				DstSeq:   3,
				HopCount: 2,
			},
			newEntry: RouteEntry{
				DstID:    1,
				DstSeq:   3,
				HopCount: 5,
			},
			wantSeq:  3,
			wantHops: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTables(t)
			if tt.existingEntry != nil {
				UpdateRoute(*tt.existingEntry)
			}
			UpdateRoute(tt.newEntry)
			got, ok := RoutesTable.Get(tt.newEntry.DstID)
			if !ok {
				t.Fatalf("route missing after UpdateRoute")
			}
			if got.HopCount != tt.wantHops || got.DstSeq != tt.wantSeq {
				t.Errorf("got HopCount=%d, DstSeq=%d; want HopCount=%d, DstSeq=%d", got.HopCount, got.DstSeq, tt.wantHops, tt.wantSeq)
			}
		})
	}
}

// There is no need to test Neighbours table because it uses the same logic as
// Route table.
