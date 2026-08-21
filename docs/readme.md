<div align="center">
  
  <h3 align="center">Mesh Network – AODV Mesh Node</h3>

  <p align="center">
    A Go implementation of an AODV (Ad-hoc On-Demand Distance Vector) like routing node with a BubbleTea TUI and CLI.
    <br />
     <a href="#usage">View Demo</a>
    ·
    <a href="#internals">Internals</a>
    <br />
  
  </p>
</div>

## About The Project

**Mesh Network** it's an academic project built to learn how mesh network works based on the principles of reactive dynamic routing of **[AODV (Ad-hoc On-Demand Distance Vector)](https://en.wikipedia.org/wiki/Ad_hoc_On-Demand_Distance_Vector_Routing)**, written in Go. It allows you to run a mesh node that can discover routes, forward messages, and maintain a dynamic network topology – all over [UDP/IP](#user-space).

This project can be used to:
- Learning how ad‑hoc routing protocols work.
- Learn how to simulate mesh networks using Docker.
- Building custom mesh applications with a clean, well‑tested codebase.

The node comes with two interfaces:
- A simple **TUI** built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) that shows live logs, neighbours and routes tables, and allows you to send messages interactively.
- A **CLI** that can send route requests, messages, and query the node’s state via a local HTTP API.



### Key Features

- **AODV Protocol mechanics** – Full support for Route Requests (RREQ), Route Replies (RREP), Route Errors (RERR), and periodic HELLO messages.
- **Dynamic Route Discovery** – Nodes find routes on‑demand, with sequence numbers to ensure freshness.
- **Precursor Lists** – Maintains a list of neighbours that depend on a route for proper RERR propagation.
- **Multi‑interface Support** – Bind to multiple network interfaces (e.g., `eth0`, `eth1`) and broadcast on all.
- **TUI** – Built with Bubble Tea and Lip Gloss; shows logs, neighbours, routes, and allows sending messages.
- **CLI** – Commands for sending messages, RREQs, and viewing node state via an IPC HTTP server.
- **Docker Compose Simulation** – Easily spin up a multi‑node network with predefined IPs and network segments.
- **Configurable** – All parameters (ID, port, interfaces, TTL, lifetime, hello interval, log level) can be set via environment variables, flags, or `.env` file.
- **Logging** – Structured logging with `slog`, writing to both terminal or a TUI and a log file.

### Built With

- [Go](https://golang.org/) – The core language.
- [Snowflake](https://github.com/bwmarrin/snowflake) – For generating unique node IDs when not provided.
- [godotenv](https://github.com/joho/godotenv) – Environment loading.
- [urfave/cli](https://github.com/urfave/cli) – CLI framework.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) – TUI framework.
- [Bubbles](https://github.com/charmbracelet/bubbles) – Reusable TUI components (table, textarea, viewport).
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) – Styling for the TUI.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- GETTING STARTED -->
## Getting Started

Follow these instructions to get a development or simulation environment running.

### Prerequisites

- **Go 1.21+** (the project uses `go 1.26.2` in `go.mod`, but older versions should work).
- **Docker** and **Docker Compose** (optional, for running the multi‑node simulation).

### Installation

#### Best way to spin up demo simulation

Clone the repository
   ```sh
   git clone https://github.com/s-588/mesh-network.git
   cd mesh-network
   ```

Run the Docker Compose simulation. This will bring up 11 nodes connected across four networks, simulating a complex topology.
   ```sh
   docker compose up --build
   ```

Network topology of this simulation looks like this:


   

#### To install as app



And build the binary
   ```sh
   go build -o mesh-node ./cmd/mesh-node
   ./mesh-node
   ```
Or just install using go
   ```sh
   go run ./cmd/mesh-node
   ```

3. **Run a single node** (default listens on `eth0`, port 6040):
   ```sh
   ```
   This starts the TUI. You can also run in daemon mode (no TUI) with `--daemon`.

4. **

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- USAGE EXAMPLES -->
## Usage

### TUI (Terminal User Interface)

When you run the node without the `--daemon` flag, the TUI opens. It is divided into:

- **Left panel**: Enter the destination node ID and the message payload, then click **Send** (or press Enter when focused on the send button).
- **Right panel**: Live logs showing messages, protocol events, and errors.
- **Bottom tables**: Neighbour table (ID, address, last seen, interface) and Routes table (destination, sequence, hops, next hop, address, interface).

**Keyboard shortcuts:**
- `Tab` / `Shift+Tab` – Move focus between elements.
- `Enter` – When focused on the send button, sends the message; when focused on an input, moves to the next field.
- `Esc` – Quit the TUI.
- `Ctrl+C` – Force quit.

### CLI Commands

The node also exposes a local HTTP API on `127.0.0.1:6242` (by default), which the CLI uses. You can invoke the CLI using the same binary:

```sh
./mesh-node send rreq <target_id>
# Sends a RREQ to discover a route to target_id.

./mesh-node send msg <target_id> "<message>"
# Sends a text message to target_id (payload is UTF‑8).

./mesh-node show messages      # or: show m / show msgs
# Displays all received messages.

./mesh-node show neighbours    # or: show n
# Shows the neighbour table with last seen times.

./mesh-node show routes        # or: show r
# Shows the routing table.
```

All commands communicate with the background node via the IPC server, so the node must be running (either in daemon mode or TUI mode) for the CLI to work.

### Environment Variables & Flags

The node can be configured using environment variables (loaded from `.env` if present) or command‑line flags. Flags take precedence.

| Environment Variable | Flag               | Description                                      | Default            |
|----------------------|--------------------|--------------------------------------------------|-------------------|
| `ID`                 | `--id`             | Node ID (Snowflake generated if not set)         | auto-generated    |
| `PORT`               | `--port`           | UDP port for all communication                   | 6040              |
| `INTERFACE`          | `--interface`      | Comma‑separated list of interfaces (e.g., `eth0,eth1`) | `eth0`        |
| `TTL`                | `--ttl`            | Maximum hops for messages (RREQ/DATA)           | 20                |
| `LIFETIME`           | `--lifetime`       | Route lifetime (seconds)                         | 30                |
| `HELLO_INTERVAL`     | `--hello-interval` | HELLO broadcast interval (seconds)               | 5                 |
| `LOG_LEVEL`          | `--log-level`      | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR`      | `INFO`            |
| `LOG_FILE`           | `--log-file`       | Path to log file (defaults to `~/mesh-network_<timestamp>.log`) | auto-generated |
| `DAEMON`             | `--daemon`         | Run without TUI (daemon mode)                    | `false`           |

Example:
```sh
ID=42 PORT=7000 INTERFACE=eth0,eth1 ./mesh-node
```

Or using flags:
```sh
./mesh-node --id 42 --port 7000 --interface eth0,eth1
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- SIMULATION WITH DOCKER COMPOSE -->
## Simulation with Docker Compose

The provided `docker-compose.yaml` defines 11 nodes with custom IP addresses across four networks (`net‑a`, `net‑b`, `net‑c`, `net‑d`). This setup emulates a real multi‑hop topology where nodes can reach each other only through intermediate nodes.

### Topology Overview

- **Node 1**: `172.20.1.10` (net‑a) and `172.20.2.14` (net‑b) – bridge between net‑a and net‑b.
- **Node 2**: `172.20.1.12` (net‑a)
- **Node 3**: `172.20.1.13` (net‑a)
- **Node 4**: `172.20.1.14` (net‑a)
- **Node 5**: `172.20.2.10` (net‑b)
- **Node 6**: `172.20.2.12` (net‑b)
- **Node 7**: `172.20.2.13` (net‑b) and `172.20.3.14` (net‑c) – bridge between net‑b and net‑c.
- **Node 8**: `172.20.3.10` (net‑c)
- **Node 9**: `172.20.3.12` (net‑c) and `172.20.1.15` (net‑a) – bridge between net‑c and net‑a (creates a loop).
- **Node 10**: `172.20.3.13` (net‑c) and `172.20.4.12` (net‑d) – bridge between net‑c and net‑d.
- **Node 11**: `172.20.4.10` (net‑d)

### Running the Simulation

1. Build the Docker images:
   ```sh
   docker compose build
   ```

2. Start all nodes:
   ```sh
   docker compose up
   ```

3. Each node will have its own TUI (if not daemonized). You can attach to any container to interact:
   ```sh
   docker attach node1
   ```

4. To stop:
   ```sh
   docker compose down
   ```

This setup is perfect for testing route discovery, message forwarding, and error handling without needing physical hardware.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- PROJECT STRUCTURE -->
## Project Structure

The code is organised as follows:

```
mesh-network/
├── cmd/
│   ├── mesh-node/          # Main entry point
│   │   └── main.go
│   ├── cli/                # CLI client and IPC server
│   │   ├── client.go       # HTTP client functions
│   │   ├── server.go       # IPC HTTP server
│   │   ├── models.go       # DTOs for JSON
│   │   └── errors.go       # CLI error definitions
│   ├── tui/                # TUI implementation
│   │   ├── model.go        # Bubble Tea model
│   │   ├── messages.go     # Tea messages and commands
│   │   ├── logger.go       # Custom slog.Handler for TUI logs
│   │   └── style.go        # Lip Gloss styles
│   └── style/              # Shared style definitions
├── internal/
│   ├── config/             # Configuration parsing (env, flags)
│   ├── protocol/           # AODV message types and serialisation
│   │   ├── messages.go
│   │   ├── errors.go
│   │   └── messages_test.go
│   ├── routing/            # Routing and neighbour tables
│   │   ├── routes.go
│   │   └── routes_test.go
│   ├── socket/             # UDP socket handling and message processing
│   │   ├── conn.go
│   │   └── conn_test.go
│   └── logger/             # Slog setup and pretty handling
├── pkg/logger/             # Log type constants (shared with tui)
├── go.mod
├── go.sum
├── docker-compose.yaml
├── .env.example
└── README.md
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- ROADMAP -->
## Roadmap

- [x] Basic AODV protocol (RREQ, RREP, RERR, HELLO)
- [x] Thread‑safe tables
- [x] TUI with live logs and tables
- [x] CLI with commands
- [x] Docker Compose simulation
- [x] Configuration via env/flags
- [ ] Support for IPv6
- [ ] Encryption / authentication (security layer)
- [ ] Persistent storage of routes (to survive restarts)
- [ ] Metrics / monitoring endpoint

See the [open issues](https://github.com/s-588/mesh-network/issues) for a full list of proposed features and known issues.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTRIBUTING -->
## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

If you have a suggestion that would make this better, please fork the repo and create a pull request. You can also simply open an issue with the tag "enhancement".
Don't forget to give the project a star! Thanks again!

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Top contributors:

<a href="https://github.com/s-588/mesh-network/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=s-588/mesh-network" alt="contrib.rocks image" />
</a>

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- LICENSE -->
## License

Distributed under the Unlicense License. See `LICENSE` for more information.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTACT -->
## Contact

Your Name – [@your_twitter](https://twitter.com/your_username) – email@example.com

Project Link: [https://github.com/s-588/mesh-network](https://github.com/s-588/mesh-network)

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- ACKNOWLEDGMENTS -->
## Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) – for making TUI development a joy.
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) – for beautiful terminal styling.
- [urfave/cli](https://github.com/urfave/cli) – for the CLI framework.
- [AODV RFC 3561](https://datatracker.ietf.org/doc/html/rfc3561) – the protocol specification.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Internals

The entire protocol (RREQ, RREP, RERR, HELLO, DATA) is implemented from scratch with binary marshalling, sequence numbers, broadcast IDs, and route expiry. The routing and neighbour tables are thread‑safe and designed for concurrent use.

## User-space

<!-- MARKDOWN LINKS & IMAGES -->
[contributors-shield]: https://img.shields.io/github/contributors/s-588/mesh-network.svg?style=for-the-badge
[contributors-url]: https://github.com/s-588/mesh-network/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/s-588/mesh-network.svg?style=for-the-badge
[forks-url]: https://github.com/s-588/mesh-network/network/members
[stars-shield]: https://img.shields.io/github/stars/s-588/mesh-network.svg?style=for-the-badge
[stars-url]: https://github.com/s-588/mesh-network/stargazers
[issues-shield]: https://img.shields.io/github/issues/s-588/mesh-network.svg?style=for-the-badge
[issues-url]: https://github.com/s-588/mesh-network/issues
[license-shield]: https://img.shields.io/github/license/s-588/mesh-network.svg?style=for-the-badge
[license-url]: https://github.com/s-588/mesh-network/blob/master/LICENSE
[linkedin-shield]: https://img.shields.io/badge/-LinkedIn-black.svg?style=for-the-badge&logo=linkedin&colorB=555
[linkedin-url]: https://linkedin.com/in/your_username
[go-shield]: https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go
[go-url]: https://golang.org/
[bubbletea-shield]: https://img.shields.io/badge/Built%20with-Bubble%20Tea-ff69b4?style=for-the-badge&logo=tea
[bubbletea-url]: https://github.com/charmbracelet/bubbletea
[product-screenshot]: images/screenshot.png
