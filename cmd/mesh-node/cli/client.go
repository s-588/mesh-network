package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/s-588/mesh-network/cmd/mesh-node/style"
)

const IPC_PORT = ":6242"

func ExecuteCLICommand(args []string) {
	cmd := args[0]
	baseURL := "http://127.0.0.1" + IPC_PORT

	switch cmd {
	case "send":
		if len(args) < 3 {
			fmt.Println("Usage: ./app send <ID> <message>")
			return
		}
		dst := args[1]
		msg := strings.Join(args[2:], " ")

		// Отправляем HTTP запрос к нашему фоновому процессу
		reqURL := fmt.Sprintf("%s/send?dst=%s&msg=%s", baseURL, dst, url.QueryEscape(msg))
		resp, err := http.Get(reqURL)
		if err != nil {
			fmt.Println("Error: can't connect to background process. Is node started?")
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		fmt.Print(string(body))

	case "rreq":
		if len(args) < 2 {
			fmt.Println(lipgloss.NewStyle().Foreground(style.Warn).Render("Usage: ./app rreq <ID>"))
			return
		}
		dst := args[1]
		resp, err := http.Get(fmt.Sprintf("%s/rreq?dst=%s", baseURL, dst))
		if err != nil {
			fmt.Println(lipgloss.NewStyle().Foreground(style.Warn).Render("Can't connect to background process."))
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Print(lipgloss.NewStyle().Foreground(style.Accent).Render(string(body)))

	case "show":
		if len(args) < 2 {
			fmt.Println(lipgloss.NewStyle().Foreground(style.Warn).Render("Available commands: show messages, show routes, show neighbours"))
			return
		}

		switch args[1] {
		case "messages":
			resp, err := http.Get(baseURL + "/messages")
			if err != nil {
				fmt.Println("Error: Can't connect to background process.")
				return
			}
			defer resp.Body.Close()

			var msgs []string
			if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
				fmt.Println("Can't decode response")
				return
			}

			if len(msgs) == 0 {
				fmt.Println("No incoming messages.")
				return
			}
			fmt.Println("=== Incomming messages ===")
			for _, m := range msgs {
				fmt.Println(m)
			}

		case "routes":
			resp, err := http.Get(baseURL + "/routes")
			if err != nil {
				fmt.Println(lipgloss.NewStyle().Foreground(style.Warn).Render("Can't connect to background process."))
				return
			}
			defer resp.Body.Close()

			var routes []RouteDTO
			if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
				fmt.Println(lipgloss.NewStyle().Foreground(style.Warn).Render("Can't decode routes table."))
				return
			}

			fmt.Println(style.TitleStyle.Render("\n Routes table "))
			if len(routes) == 0 {
				fmt.Println(lipgloss.NewStyle().Foreground(style.Muted).Render("Empty table."))
				return
			}

			// Настраиваем фиксированную ширину колонок через lipgloss для ровного отображения
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
			fmt.Println(header)
			fmt.Println(lipgloss.NewStyle().Foreground(style.Muted).Render(strings.Repeat("─", 65)))

			for _, r := range routes {
				row := lipgloss.JoinHorizontal(lipgloss.Left,
					c1(lipgloss.NewStyle().Foreground(style.Accent).Render(fmt.Sprint(r.DstID))),
					c2(lipgloss.NewStyle().Foreground(style.TextColor).Render(r.NextHop)),
					c3(lipgloss.NewStyle().Foreground(style.TextColor).Render(fmt.Sprint(r.Hops))),
					c4(lipgloss.NewStyle().Foreground(style.TextColor).Render(fmt.Sprint(r.Seq))),
					c5(lipgloss.NewStyle().Foreground(style.Accent2).Render(r.Iface)),
				)
				fmt.Println(row)
			}
			fmt.Println()

		case "neighbours":
			resp, err := http.Get(baseURL + "/neighbours")
			if err != nil {
				fmt.Println(lipgloss.NewStyle().Foreground(style.Warn).Render("Can't connect to background proccess."))
				return
			}
			defer resp.Body.Close()

			var neighbours []NeighDTO
			if err := json.NewDecoder(resp.Body).Decode(&neighbours); err != nil {
				fmt.Println(lipgloss.NewStyle().Foreground(style.Warn).Render("Error decoding neighbours table."))
				return
			}

			fmt.Println(style.TitleStyle.Render("\n Neighbours table "))
			if len(neighbours) == 0 {
				fmt.Println(lipgloss.NewStyle().Foreground(style.Muted).Render("Empty table."))
				return
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
			fmt.Println(header)
			fmt.Println(lipgloss.NewStyle().Foreground(style.Muted).Render(strings.Repeat("─", 65)))

			for _, n := range neighbours {
				row := lipgloss.JoinHorizontal(lipgloss.Left,
					c1(lipgloss.NewStyle().Foreground(style.Accent).Render(fmt.Sprint(n.ID))),
					c2(lipgloss.NewStyle().Foreground(style.TextColor).Render(n.Addr)),
					c3(lipgloss.NewStyle().Foreground(style.Warn).Render(n.LastSeen)),
					c4(lipgloss.NewStyle().Foreground(style.Accent2).Render(n.Iface)),
				)
				fmt.Println(row)
			}
			fmt.Println()

		default:
			fmt.Println(lipgloss.NewStyle().Foreground(style.Warn).Render("Uknown subcommand for show."))
		}

	default:
		fmt.Println(style.TitleStyle.Render("Help:"))
		fmt.Println(style.LabelStyle.Render("  ./app send <ID> <message>") + lipgloss.NewStyle().Foreground(style.TextColor).Render("\t- send message"))
		fmt.Println(style.LabelStyle.Render("  ./app rreq <ID>") + lipgloss.NewStyle().Foreground(style.TextColor).Render("\t- send route request"))
		fmt.Println(style.LabelStyle.Render("  ./app show messages") + lipgloss.NewStyle().Foreground(style.TextColor).Render("\t- show incomming messages"))
		fmt.Println(style.LabelStyle.Render("  ./app show routes") + lipgloss.NewStyle().Foreground(style.TextColor).Render("\t- show active routes"))
		fmt.Println(style.LabelStyle.Render("  ./app show neighbours") + lipgloss.NewStyle().Foreground(style.TextColor).Render("\t- show neighbours"))
	}
}
