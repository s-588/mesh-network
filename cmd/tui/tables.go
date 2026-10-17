package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

const (
	colID = iota
	colAddr
	colSeen
	colIface
	colSeq
	colHops
	colNext
)

// bubbles table Cell/Header styles add Padding(0, 1) around each column.
const colPad = 2

type colSpec struct {
	id                   int
	title, short         string
	min, max, flex, drop int
}

func (m *model) rebuildTables() {
	sort.SliceStable(m.neighbours, func(i, j int) bool {
		return m.neighbours[i].sortID < m.neighbours[j].sortID
	})
	sort.SliceStable(m.routes, func(i, j int) bool {
		return m.routes[i].sortID < m.routes[j].sortID
	})

	nCols, nIDs, nWidths := fitColumns(m.neighTableW, neighbourSpecs())
	rCols, rIDs, rWidths := fitColumns(m.routeTableW, routeSpecs())

	nRows := make([]table.Row, 0, len(m.neighbours))
	for _, n := range m.neighbours {
		row := make(table.Row, len(nIDs))
		for i, id := range nIDs {
			row[i] = n.cell(id, nWidths[i])
		}
		nRows = append(nRows, row)
	}
	rRows := make([]table.Row, 0, len(m.routes))
	for _, r := range m.routes {
		row := make(table.Row, len(rIDs))
		for i, id := range rIDs {
			row[i] = r.cell(id, rWidths[i])
		}
		rRows = append(rRows, row)
	}

	// Clear rows before swapping columns so the table never renders a short row.
	m.neighboursTable.SetRows(nil)
	m.neighboursTable.SetColumns(nCols)
	m.neighboursTable.SetWidth(max(m.neighTableW, 1))
	m.neighboursTable.SetHeight(max(m.neighTableH, 3))
	m.neighboursTable.SetRows(nRows)

	m.routesTable.SetRows(nil)
	m.routesTable.SetColumns(rCols)
	m.routesTable.SetWidth(max(m.routeTableW, 1))
	m.routesTable.SetHeight(max(m.routeTableH, 3))
	m.routesTable.SetRows(rRows)
}

func shortSeen(seen string, width int) string {
	if width >= 16 || seen == "" {
		return seen
	}
	if i := strings.LastIndex(seen, " "); i >= 0 && i+1 < len(seen) {
		return seen[i+1:]
	}
	return seen
}

// fitColumns drops low-priority columns, then distributes leftover width.
// The returned widths are content widths; rendered size is width+colPad each.
//
//nolint:gocyclo // function is complex but clear and understandable
func fitColumns(total int, specs []colSpec) (cols []table.Column, ids []int, widths []int) {
	if total < 1 {
		total = 1
	}
	active := append([]colSpec(nil), specs...)
	for len(active) > 1 && columnCost(active) > total {
		dropAt := 0
		for i := 1; i < len(active); i++ {
			if active[i].drop > active[dropAt].drop {
				dropAt = i
			}
		}
		active = append(active[:dropAt], active[dropAt+1:]...)
	}

	widths = make([]int, len(active))
	for i, s := range active {
		widths[i] = s.min
		if widths[i] > s.max {
			widths[i] = s.max
		}
		if widths[i] < 1 {
			widths[i] = 1
		}
	}

	budget := total - colPad*len(active)
	if budget < len(active) {
		budget = len(active)
	}
	for sumInts(widths) > budget {
		idx := -1
		for i, w := range widths {
			if w > 1 && (idx == -1 || w > widths[idx]) {
				idx = i
			}
		}
		if idx == -1 {
			break
		}
		widths[idx]--
	}

	extra := budget - sumInts(widths)
	for extra > 0 {
		moved := false
		for i, s := range active {
			if extra == 0 {
				break
			}
			if s.flex > 0 && widths[i] < s.max {
				widths[i]++
				extra--
				moved = true
			}
		}
		if !moved {
			break
		}
	}

	cols = make([]table.Column, len(active))
	ids = make([]int, len(active))
	for i, s := range active {
		title := s.title
		if lipgloss.Width(title) > widths[i] {
			title = s.short
		}
		cols[i] = table.Column{Title: title, Width: widths[i]}
		ids[i] = s.id
	}
	return cols, ids, widths
}

func columnCost(specs []colSpec) int {
	n := 0
	for _, s := range specs {
		w := s.min
		if w < 1 {
			w = 1
		}
		n += w + colPad
	}
	return n
}

func sumInts(xs []int) int {
	n := 0
	for _, v := range xs {
		n += v
	}
	return n
}
