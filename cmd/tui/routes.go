package tui

import (
	"context"
	"fmt"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/cmd/style"
	"github.com/s-588/mesh-network/internal/ipc"
	"github.com/s-588/mesh-network/internal/routing"
)

type routeEntry struct {
	dst, seq, hops, next, addr, iface string
	sortID                            uint64
}

func (m model) renderRoutesBlock() string {
	return m.renderTablePanel(m.routesW, m.routesH, m.renderRoutesInner())
}

func (m model) renderRoutesInner() string {
	w := max(m.routesW-frameX, 1)
	st := style.ResultHeaderStyle
	if m.focused == focusRoutesTable {
		st = style.TitleStyle
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		oneLine(st.Render("Routes"), w),
		m.routesTable.View(),
	)
}

func (m *model) refreshRoutesTable() {
	if m.sock != nil {
		entries := routing.RoutesTable.Snapshot()
		rows := make([]routeEntry, 0, len(entries))
		for _, e := range entries {
			rows = append(rows, routeEntry{
				dst:    fmt.Sprintf("%d", e.DstID),
				seq:    fmt.Sprintf("%d", e.DstSeq),
				hops:   fmt.Sprintf("%d", e.HopCount),
				next:   fmt.Sprintf("%d", e.NextHopID),
				addr:   e.NextHopAddr.String(),
				iface:  e.Interface,
				sortID: e.DstID,
			})
		}
		m.routes = rows
		m.rebuildTables()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	list, err := ipc.GetRoutes(ctx)
	if err != nil {
		return
	}
	rows := make([]routeEntry, 0, len(list))
	for _, r := range list {
		rows = append(rows, routeEntry{
			dst:    fmt.Sprintf("%d", r.DstID),
			seq:    fmt.Sprintf("%d", r.Seq),
			hops:   fmt.Sprintf("%d", r.Hops),
			next:   fmt.Sprintf("%d", r.NextHopID),
			addr:   r.NextHop,
			iface:  r.Iface,
			sortID: r.DstID,
		})
	}
	m.routes = rows
	m.rebuildTables()
}

func (r routeEntry) cell(id, width int) string {
	switch id {
	case colSeq:
		return r.seq
	case colHops:
		return r.hops
	case colNext:
		return r.next
	case colAddr:
		return r.addr
	case colIface:
		return r.iface
	default:
		return r.dst
	}
}

func routeSpecs() []colSpec {
	return []colSpec{
		{id: colID, title: "Dst", short: "Dst", min: 4, max: 10, flex: 1, drop: 0},
		{id: colSeq, title: "Seq", short: "Sq", min: 3, max: 8, flex: 1, drop: 5},
		{id: colHops, title: "Hops", short: "Hp", min: 3, max: 5, flex: 1, drop: 4},
		{id: colNext, title: "Next Hop", short: "Via", min: 4, max: 10, flex: 2, drop: 1},
		{id: colAddr, title: "Address", short: "Addr", min: 8, max: 24, flex: 3, drop: 6},
		{id: colIface, title: "Iface", short: "If", min: 4, max: 12, flex: 1, drop: 3},
	}
}
