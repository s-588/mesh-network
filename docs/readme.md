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

<img alt="Docker Compose test topology" src="https://github.com/user-attachments/assets/63381571-041b-4c04-8fc7-bcbc5f2f0e75" />

#### To install as app

  ```sh
   go install github.com/s-588/mesh-network/cmd/mesh-node@latest
   ```

Or clone and build
   ```sh
   git clone https://github.com/s-588/mesh-network.git
   cd mesh-network
   go build -o mesh-node ./cmd/mesh-node
   ```

## Usage

### Docker Compose simulation

To enter a node interface you need to attach terminal to a container.
```sh
docker compose attach node2
```

Logs are placed in the `/root/` folder.

To CLI in container you can attach it with /bin/sh
```sh
docker exec -it node2 /bin/sh
```
And now you can use [CLI](#CLI) to interact with node
```sh
mesh-node --help
```

### TUI (Terminal User Interface)

When you run the node without the `--daemon` flag, the TUI opens. It is divided into:

- **Left panel**: Enter the destination node ID and the message payload, then click **Send** (or press Enter when focused on the send button).
- **Right panel**: Live logs showing messages, protocol events, and errors.
- **Bottom tables**: Neighbours table (ID, address, last seen, interface) and Routes table (destination, sequence, hops, next hop, address, interface).

**Keyboard shortcuts:**
- `Tab` / `Shift+Tab` – Move focus between elements.
- `Enter` – When focused on the send button, sends the message; when focused on an input, moves to the next field.
- `Esc` / `Ctrl+C` – Force quit.

### CLI Commands

The node also exposes a local HTTP API on `127.0.0.1:6242` (by default), which the CLI uses. You can invoke the CLI using the same binary:

Sends a RREQ to discover a route to target_id.
```sh
./mesh-node send rreq <target_id>
```

Sends a text message to target_id (payload is UTF‑8).
```sh
./mesh-node send msg <target_id> "<message>"
```

Displays all received messages.
```sh
./mesh-node show messages
```

Shows the neighbour table with last seen times.
```sh
./mesh-node show neighbours
```

Shows the routing table.
```sh
./mesh-node show routes  
```

You can find additional information by running 
```sh
./mesh-node --help 
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
| `LOG_FILE`           | `--log-file`       | Path to log file (defaults to `~/<timestamp>.log`) | auto-generated |
| `DAEMON`             | `--daemon`         | Run without TUI (daemon mode)                    | `false`           |

Example:
```sh
ID=42 PORT=7000 INTERFACE=eth0,eth1 ./mesh-node
```

Or using flags:
```sh
./mesh-node --id 42 --port 7000 --interface eth0,eth1
```

## Internals

When I started this project I wanted to create routing protocol and learn how mesh networks work on practice. So I research what are the existing solutions for mesh networks and found out bunch of routing protocols like [RIP](https://en.wikipedia.org/wiki/Routing_Information_Protocol) and [OSPF](https://en.wikipedia.org/wiki/Open_Shortest_Path_First) and specifically designed for dynamic networks [AODV](https://en.wikipedia.org/wiki/Ad_hoc_On-Demand_Distance_Vector_Routing), [DSR](https://en.wikipedia.org/wiki/Dynamic_Source_Routing), [OLSR](https://en.wikipedia.org/wiki/Optimized_Link_State_Routing_Protocol). I reviewed them, how they work and decided that most interesting will be implement something that look like AODV. So during implementation I didn't read [RFC 3561](https://datatracker.ietf.org/doc/rfc3561/) because I wanted to came up with my own solution without understanding of what happening under the hood of AODV and I believe I did well.   

### User-space

Mesh network created by the app it's a logical mesh network. So transportation of packages trusted to UDP/IP stack, this means that all nodes on the same network can communicate without any help from my application. To solve this problem I decided to support multiple interfaces, this forces app to route packages between networks and allows to create something that look like real mesh network.
