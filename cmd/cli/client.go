package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/cmd/style"
)

// IPC_PORT is a port where active local node is listening.
const IPC_PORT = ":6242"

var (
	baseURL = url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("127.0.0.1", IPC_PORT),
	}

	ErrNodeUnattainable = errors.New("can't connect to background process. Is node started?")
	ErrNoRecords        = errors.New("no records")
)

// SendRREQ is a helper function for sending HTTP request to IPC node server
func SendRREQ(dst int64) (string, error) {
	baseURL.Path = "rreq"
	val := baseURL.Query()
	val.Set("dst", strconv.FormatInt(dst, 64))
	baseURL.RawQuery = val.Encode()
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return "", ErrNodeUnattainable
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

// SendMsg send request to send text data to node's IPC.
func SendMsg(dst int64, msg string) (string, error) {
	baseURL.Path = "send"
	val := baseURL.Query()
	val.Set("dst", strconv.FormatInt(dst, 64))
	val.Set("msg", url.QueryEscape(msg))
	baseURL.RawQuery = val.Encode()
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return "", ErrNodeUnattainable
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

// GetMsg recieve messages from node's IPC server.
func GetMsgs() (string, error) {
	baseURL.Path = "/messages"
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return "", ErrNodeUnattainable
	}
	defer resp.Body.Close()

	var msgs []string
	if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
		return "", errors.New("Can't decode response")
	}

	if len(msgs) == 0 {
		return "", ErrNoRecords
	}
	return strings.Join(msgs, "\n"), nil
}

func GetNeighbours() (string, error) {
	s := strings.Builder{}
	baseURL.Scheme = "/neighbours"
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var neighbours []NeighDTO
	if err := json.NewDecoder(resp.Body).Decode(&neighbours); err != nil {
		return "", err
	}

	s.WriteString(style.TitleStyle.Render("\n Neighbours table "))
	if len(neighbours) == 0 {
		return "", ErrNoRecords
	}

	c1 := lipgloss.NewStyle().Width(10).Render
	c2 := lipgloss.NewStyle().Width(25).Render
	c3 := lipgloss.NewStyle().Width(20).Render
	c4 := lipgloss.NewStyle().Width(10).Render

	header := lipgloss.JoinHorizontal(lipgloss.Left,
		c1(style.ResultHeaderStyle.Render("ID")),
		c2(style.ResultHeaderStyle.Render("Address")),
		c3(style.ResultHeaderStyle.Render("Last Seen")),
		c4(style.ResultHeaderStyle.Render("Iface")),
	)
	s.WriteString(header)
	s.WriteString(lipgloss.NewStyle().Foreground(style.Muted).Render(strings.Repeat("─", 65)))

	for _, n := range neighbours {
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			c1(lipgloss.NewStyle().Foreground(style.Accent).Render(fmt.Sprint(n.ID))),
			c2(lipgloss.NewStyle().Foreground(style.TextColor).Render(n.Addr)),
			c3(lipgloss.NewStyle().Foreground(style.Warn).Render(n.LastSeen)),
			c4(lipgloss.NewStyle().Foreground(style.Accent2).Render(n.Iface)),
		)
		s.WriteString(row)
	}
	return s.String(), nil
}

func GetRoutes() (string, error) {
	s := strings.Builder{}
	baseURL.Path = "/routes"
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var routes []RouteDTO
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return "", err
	}

	s.WriteString(style.TitleStyle.Render("\n Routes table "))
	if len(routes) == 0 {
		return "", ErrNoRecords
	}

	c1 := lipgloss.NewStyle().Width(10).Render
	c2 := lipgloss.NewStyle().Width(25).Render
	c3 := lipgloss.NewStyle().Width(8).Render
	c4 := lipgloss.NewStyle().Width(8).Render
	c5 := lipgloss.NewStyle().Width(10).Render

	header := lipgloss.JoinHorizontal(lipgloss.Left,
		c1(style.ResultHeaderStyle.Render("DstID")),
		c2(style.ResultHeaderStyle.Render("NextHop")),
		c3(style.ResultHeaderStyle.Render("Hops")),
		c4(style.ResultHeaderStyle.Render("Seq")),
		c5(style.ResultHeaderStyle.Render("Iface")),
	)
	s.WriteString(header)
	s.WriteString(lipgloss.NewStyle().Foreground(style.Muted).Render(strings.Repeat("─", 65)))

	for _, r := range routes {
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			c1(lipgloss.NewStyle().Foreground(style.Accent).Render(fmt.Sprint(r.DstID))),
			c2(lipgloss.NewStyle().Foreground(style.TextColor).Render(r.NextHop)),
			c3(lipgloss.NewStyle().Foreground(style.TextColor).Render(fmt.Sprint(r.Hops))),
			c4(lipgloss.NewStyle().Foreground(style.TextColor).Render(fmt.Sprint(r.Seq))),
			c5(lipgloss.NewStyle().Foreground(style.Accent2).Render(r.Iface)),
		)
		s.WriteString(row)
	}
	return s.String(), nil
}
