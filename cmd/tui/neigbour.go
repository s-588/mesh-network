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

type neighbour struct {
	id, addr, seen, iface string
	sortID                uint64
}

func (m model) renderNeighboursBlock() string {
	return m.renderTablePanel(m.neighW, m.neighH, m.renderNeighInner())
}

func (m model) renderNeighInner() string {
	w := max(m.neighW-frameX, 1)
	st := style.ResultHeaderStyle
	if m.focused == focusNeighboursTable {
		st = style.TitleStyle
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		oneLine(st.Render("Neighbours"), w),
		m.neighboursTable.View(),
	)
}

func (m *model) refreshNeighboursTable() {
	if m.sock != nil {
		entries := routing.NeighboursTable.Snapshot()
		rows := make([]neighbour, 0, len(entries))
		for _, e := range entries {
			seen := "never"
			if !e.LastSeen.IsZero() {
				seen = e.LastSeen.Format(time.DateTime)
			}
			rows = append(rows, neighbour{
				id:     fmt.Sprintf("%d", e.ID),
				addr:   e.Addr.String(),
				seen:   seen,
				iface:  e.Interface,
				sortID: e.ID,
			})
		}
		m.neighbours = rows
		m.rebuildTables()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	list, err := ipc.GetNeighbours(ctx)
	if err != nil {
		return
	}
	rows := make([]neighbour, 0, len(list))
	for _, n := range list {
		rows = append(rows, neighbour{
			id:     fmt.Sprintf("%d", n.ID),
			addr:   n.Addr,
			seen:   n.LastSeen,
			iface:  n.Iface,
			sortID: n.ID,
		})
	}
	m.neighbours = rows
	m.rebuildTables()
}

func (n neighbour) cell(id, width int) string {
	switch id {
	case colAddr:
		return n.addr
	case colSeen:
		return shortSeen(n.seen, width)
	case colIface:
		return n.iface
	default:
		return n.id
	}
}

func neighbourSpecs() []colSpec {
	return []colSpec{
		{id: colID, title: "ID", short: "ID", min: 4, max: 20, flex: 1, drop: 0},
		{id: colAddr, title: "Address", short: "Addr", min: 8, max: 24, flex: 3, drop: 2},
		{id: colSeen, title: "Last Seen", short: "Seen", min: 8, max: 19, flex: 2, drop: 4},
		{id: colIface, title: "Iface", short: "If", min: 4, max: 12, flex: 1, drop: 3},
	}
}
