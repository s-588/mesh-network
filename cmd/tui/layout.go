package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/cmd/style"
)

// Layout modes. Picked from the real terminal size — never inflated.
const (
	layoutWide layoutMode = iota
	layoutSplit
	layoutStack
	layoutCompact
	layoutTiny
)

const (
	wideMinWidth    = 100
	wideMinHeight   = 26
	splitMinWidth   = 64
	splitMinHeight  = 22
	stackMinWidth   = 36
	stackMinHeight  = 30
	tinyBelowWidth  = 24
	tinyBelowHeight = 10

	// PanelStyle is a rounded border plus horizontal padding of 1.
	// Dense mode drops the vertical padding (see frameY).
	frameX = 4

	// FocusFieldStyle: horizontal padding 1 + border.
	fieldFrameX = 4

	buttonRowW = 12
	buttonGap  = 2
	inlineMinW = fieldFrameX + 8 + buttonGap + buttonRowW // 26
)

type layoutMode int

func (m model) frameY() int {
	if m.dense {
		return 2 // border only; vertical padding removed
	}
	return 4 // border + Padding(1, 1)
}

func (m model) panel(right bool) lipgloss.Style {
	s := style.LeftPanelStyle
	if right {
		s = style.RightPanelStyle
	}
	if m.dense {
		s = s.Padding(0, 1)
	}
	return s
}

func (m model) renderPanel(width, height int, right bool, content string) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return m.panel(right).Width(width).Height(height).Render(content)
}

func (m model) renderTablePanel(width, height int, content string) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	s := style.PanelStyle
	if m.dense {
		s = s.Padding(0, 1)
	}
	return s.Width(width).Height(height).Render(content)
}

// updateSizes picks a layout that fits the real terminal and resizes every
// widget. Nothing is allowed to demand more rows or columns than we have.
func (m *model) updateSizes(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	m.width = width
	m.height = height
	m.mode = pickMode(width, height)
	// Comfortable panel padding needs the taller minimums. Below 36 rows
	// the vertical padding is dropped so 24-line terminals still fit.
	m.dense = height < 36
	m.slimButton = false
	m.hideComposeTitle = false
	m.showHint = false

	switch m.mode {
	case layoutWide:
		m.layoutWide()
	case layoutSplit:
		m.layoutSplit()
	case layoutStack:
		m.layoutStack()
	case layoutCompact:
		m.layoutCompact()
	default:
		m.layoutTiny()
	}
	m.applyWidgetSizes()
}

func pickMode(width, height int) layoutMode {
	switch {
	case width < tinyBelowWidth || height < tinyBelowHeight:
		return layoutTiny
	case width >= wideMinWidth && height >= wideMinHeight:
		return layoutWide
	case width >= splitMinWidth && height >= splitMinHeight:
		return layoutSplit
	case width >= stackMinWidth && height >= stackMinHeight:
		return layoutStack
	default:
		return layoutCompact
	}
}

func (m *model) layoutWide() {
	m.placeSideBySide(42, 30)
	m.neighW = m.width
	m.routesW = m.width
	m.neighTableW = max(m.neighW-frameX, 1)
	m.routeTableW = max(m.routesW-frameX, 1)

	ta, nH, rH := 1, 3, 3
	for ta < 3 && m.wideTotal(ta+1, nH, rH) <= m.height {
		ta++
	}
	for {
		grew := false
		if nH <= rH && m.wideTotal(ta, nH+1, rH) <= m.height {
			nH++
			grew = true
		}
		if m.wideTotal(ta, nH, rH+1) <= m.height {
			rH++
			grew = true
		}
		if !grew {
			break
		}
	}
	for ta < 8 && m.wideTotal(ta+1, nH, rH) <= m.height {
		ta++
	}
	m.commitTopAndTables(ta, nH, rH, true)
}

func (m *model) layoutSplit() {
	m.composeW = m.width
	m.logsW = m.width
	gap := 1
	left := (m.width - gap) / 2
	right := m.width - gap - left
	if left < 1 {
		left = 1
	}
	if right < 1 {
		right = 1
	}
	m.neighW = left
	m.routesW = right
	m.sizeComposeFields(max(m.composeW-frameX, 1))
	m.viewportWidth = max(m.logsW-frameX, 1)
	m.neighTableW = max(m.neighW-frameX, 1)
	m.routeTableW = max(m.routesW-frameX, 1)

	ta, vp, tableH := 1, 1, 3
	for ta < 2 && m.splitTotal(ta+1, vp, tableH) <= m.height {
		ta++
	}
	for vp < 4 && m.splitTotal(ta, vp+1, tableH) <= m.height {
		vp++
	}
	for ta < 4 && m.splitTotal(ta+1, vp, tableH) <= m.height {
		ta++
	}
	for vp < 8 && m.splitTotal(ta, vp+1, tableH) <= m.height {
		vp++
	}
	for m.splitTotal(ta, vp, tableH+1) <= m.height {
		tableH++
	}
	for m.splitTotal(ta, vp+1, tableH) <= m.height {
		vp++
	}
	for ta < 8 && m.splitTotal(ta+1, vp, tableH) <= m.height {
		ta++
	}
	m.commitCompose(ta)
	m.commitLogs(vp)
	m.neighTableH = tableH
	m.routeTableH = tableH
	m.neighH = 1 + tableH + m.frameY()
	m.routesH = m.neighH
}

func (m *model) layoutStack() {
	m.composeW = m.width
	m.logsW = m.width
	m.neighW = m.width
	m.routesW = m.width
	m.sizeComposeFields(max(m.composeW-frameX, 1))
	m.viewportWidth = max(m.logsW-frameX, 1)
	m.neighTableW = max(m.neighW-frameX, 1)
	m.routeTableW = max(m.routesW-frameX, 1)

	ta, vp, nH, rH := 1, 1, 3, 3
	for vp < 4 && m.stackTotal(ta, vp+1, nH, rH) <= m.height {
		vp++
	}
	for ta < 3 && m.stackTotal(ta+1, vp, nH, rH) <= m.height {
		ta++
	}
	for {
		grew := false
		if nH <= rH && m.stackTotal(ta, vp, nH+1, rH) <= m.height {
			nH++
			grew = true
		}
		if m.stackTotal(ta, vp, nH, rH+1) <= m.height {
			rH++
			grew = true
		}
		if !grew {
			break
		}
	}
	for m.stackTotal(ta, vp+1, nH, rH) <= m.height {
		vp++
	}
	for ta < 8 && m.stackTotal(ta+1, vp, nH, rH) <= m.height {
		ta++
	}
	m.commitCompose(ta)
	m.commitLogs(vp)
	m.neighTableH = nH
	m.routeTableH = rH
	m.neighH = 1 + nH + m.frameY()
	m.routesH = 1 + rH + m.frameY()
}

func (m *model) layoutCompact() {
	m.slimButton = true
	m.showHint = m.height >= 14
	hint := 0
	if m.showHint {
		hint = 1
	}
	panelH := m.height - 1 - hint
	if panelH < 3 {
		panelH = 3
	}
	panelW := max(m.width, 1)
	m.composeW, m.composeH = panelW, panelH
	m.logsW, m.logsH = panelW, panelH
	m.neighW, m.neighH = panelW, panelH
	m.routesW, m.routesH = panelW, panelH

	contentH := max(panelH-m.frameY(), 1)
	contentW := max(panelW-frameX, 1)
	m.stackSend = true
	m.hideComposeTitle = false

	fixed := composeFixedLines(true, true, true)
	if contentH-fixed < 1 {
		m.hideComposeTitle = true
		fixed = composeFixedLines(false, true, true)
	}
	ta := contentH - fixed
	if ta < 1 {
		ta = 1
	}
	if ta > 8 {
		ta = 8
	}
	m.textareaHeight = ta
	m.sizeComposeFields(contentW)

	m.viewportWidth = contentW
	m.viewportHeight = max(contentH-1, 1)
	m.neighTableW = contentW
	m.routeTableW = contentW
	m.neighTableH = max(contentH-1, 3)
	m.routeTableH = m.neighTableH
}

func (m *model) layoutTiny() {
	m.inputWidth = 4
	m.textareaWidth = 4
	m.textareaHeight = 1
	m.viewportWidth = 4
	m.viewportHeight = 1
	m.neighTableW, m.routeTableW = 4, 4
	m.neighTableH, m.routeTableH = 3, 3
}

// placeSideBySide sizes the compose | logs row. wantInput is the preferred
// text-input width; minRight is the smallest acceptable logs panel.
func (m *model) placeSideBySide(wantInput, minRight int) {
	gap := 1
	content := wantInput + fieldFrameX + buttonGap + buttonRowW
	panel := content + frameX
	if panel+gap+minRight > m.width {
		panel = m.width - gap - minRight
	}
	minPanel := inlineMinW + frameX
	if panel < minPanel && m.width-gap-16 >= minPanel {
		panel = minPanel
	}
	if panel < frameX+8 {
		panel = max(m.width/2, 1)
	}
	if panel > m.width-gap-10 {
		panel = max(m.width-gap-10, 1)
	}
	m.composeW = panel
	m.logsW = m.width - gap - panel
	if m.logsW < 1 {
		m.logsW = 1
		m.composeW = max(m.width-gap-1, 1)
	}
	m.sizeComposeFields(max(m.composeW-frameX, 1))
	m.viewportWidth = max(m.logsW-frameX, 1)
}

func (m *model) sizeComposeFields(contentW int) {
	if contentW < 1 {
		contentW = 1
	}
	if !m.slimButton {
		m.stackSend = contentW < inlineMinW
	}
	m.textareaWidth = max(contentW-fieldFrameX, 1)
	if m.stackSend {
		m.inputWidth = m.textareaWidth
		if m.inputWidth > 48 {
			m.inputWidth = 48
		}
		return
	}
	maxField := contentW - buttonGap - buttonRowW
	m.inputWidth = max(maxField-fieldFrameX, 1)
	if m.inputWidth > 48 {
		m.inputWidth = 48
	}
}

func composeFixedLines(title, stack, slim bool) int {
	n := 0
	if title {
		n++
	}
	switch {
	case stack && slim:
		n += 4 // bordered input (3) + flat button (1)
	case stack:
		n += 6 // bordered input + bordered button
	default:
		n += 3 // input and button share a row
	}
	n += 2 // textarea border
	return n
}

func (m *model) wideTotal(ta, nH, rH int) int {
	fy := m.frameY()
	top := composeFixedLines(true, m.stackSend, false) + ta + fy
	return top + (1 + nH + fy) + (1 + rH + fy) + 2
}

func (m *model) splitTotal(ta, vp, tableH int) int {
	fy := m.frameY()
	compose := composeFixedLines(true, m.stackSend, false) + ta + fy
	logs := 1 + vp + fy
	band := 1 + tableH + fy
	return compose + logs + band + 2
}

func (m *model) stackTotal(ta, vp, nH, rH int) int {
	fy := m.frameY()
	compose := composeFixedLines(true, m.stackSend, false) + ta + fy
	logs := 1 + vp + fy
	return compose + logs + (1 + nH + fy) + (1 + rH + fy) + 3
}

func (m *model) commitTopAndTables(ta, nH, rH int, logsFollowTop bool) {
	m.commitCompose(ta)
	if logsFollowTop {
		m.logsH = m.composeH
		m.viewportHeight = max(m.logsH-m.frameY()-1, 1)
	}
	m.neighTableH = nH
	m.routeTableH = rH
	m.neighH = 1 + nH + m.frameY()
	m.routesH = 1 + rH + m.frameY()
}

func (m *model) commitCompose(ta int) {
	if ta < 1 {
		ta = 1
	}
	m.textareaHeight = ta
	title := !m.hideComposeTitle
	m.composeH = composeFixedLines(title, m.stackSend, m.slimButton) + ta + m.frameY()
}

func (m *model) commitLogs(vp int) {
	if vp < 1 {
		vp = 1
	}
	m.viewportHeight = vp
	m.logsH = 1 + vp + m.frameY()
}

func (m *model) applyWidgetSizes() {
	if m.inputWidth < 1 {
		m.inputWidth = 1
	}
	if m.textareaWidth < 1 {
		m.textareaWidth = 1
	}
	if m.textareaHeight < 1 {
		m.textareaHeight = 1
	}
	if m.viewportWidth < 1 {
		m.viewportWidth = 1
	}
	if m.viewportHeight < 1 {
		m.viewportHeight = 1
	}
	m.nodeIDTextInput.SetWidth(m.inputWidth)
	m.payloadAreaInput.SetWidth(m.textareaWidth)
	m.payloadAreaInput.SetHeight(m.textareaHeight)
	m.resultViewport.SetWidth(m.viewportWidth)
	m.resultViewport.SetHeight(m.viewportHeight)
	m.rebuildTables()
	m.reloadLogs()
}
