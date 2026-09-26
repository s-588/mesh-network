[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://www.docker.com/)
[![Bubble Tea](https://img.shields.io/badge/Bubble_Tea-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://github.com/charmbracelet/bubbletea)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge)](LICENSE)
# Mesh Network – AODV Mesh Node

A Go implementation of an AODV-style ad-hoc routing node with a Bubble Tea TUI and CLI. Useful for learning reactive mesh routing and simulating multi-node topologies with Docker.

## Key Features

- Full AODV-like mechanics: RREQ, RREP, RERR, periodic HELLO
- On-demand route discovery with sequence numbers and precursor lists
- Multi-interface support (broadcast on all configured interfaces)
- Interactive Bubble Tea TUI (logs, neighbours, routes, send messages)
- CLI that talks to a local HTTP IPC server
- Ready-to-run Docker Compose multi-node simulation

## Built With

- Go 1.26+
- Bubble Tea + Bubbles + Lip Gloss (TUI)
- urfave/cli, godotenv, snowflake
- Structured logging with `slog`

## Getting Started

### Prerequisites

- Go 1.26+
- Docker & Docker Compose (for the simulation)

### Quick Start – Multi-node Simulation

```bash
git clone https://github.com/s-588/mesh-network.git
cd mesh-network
docker compose up --build
```

This starts 11 nodes across four networks. Attach to a node:

```bash
docker compose attach node2
```

Network topology of this simulation looks like this:

<img alt="Docker Compose test topology" src="https://github.com/user-attachments/assets/63381571-041b-4c04-8fc7-bcbc5f2f0e75" />


### Install / Build Locally

```bash
go install github.com/s-588/mesh-network/cmd/mesh-node@latest
# or
go build -o mesh-node ./cmd/mesh-node
```

## Usage

### TUI

Run without `--daemon` to open the TUI:

- Left: destination ID + message payload
- Right: live logs
- Bottom: neighbours & routes tables

Keyboard: `Tab` / `Shift+Tab` to navigate, `Enter` to send, `Esc` / `Ctrl+C` to quit.

### CLI (requires a running node)

```bash
./mesh-node send rreq <target_id>
./mesh-node send msg <target_id> "hello"
./mesh-node show messages
./mesh-node show neighbours
./mesh-node show routes
./mesh-node --help
```

### Configuration

| Env / Flag            | Description                          | Default          |
|-----------------------|--------------------------------------|------------------|
| `ID` / `--id`         | Node ID                              | auto (Snowflake) |
| `PORT` / `--port`     | UDP port                             | 6040             |
| `INTERFACE` / `--interface` | Comma-separated interfaces     | `eth0`           |
| `TTL` / `--ttl`       | Max hops                             | 20               |
| `LIFETIME` / `--lifetime` | Route lifetime (s)               | 30               |
| `HELLO_INTERVAL` / `--hello-interval` | HELLO interval (s)     | 5                |
| `LOG_LEVEL` / `--log-level` | DEBUG / INFO / WARN / ERROR    | INFO             |
| `DAEMON` / `--daemon` | Run without TUI                      | false            |

## Internals

When I started this project I wanted to create routing protocol and learn how mesh networks work on practice. So I research what are the existing solutions for mesh networks and found out bunch of routing protocols like [RIP](https://en.wikipedia.org/wiki/Routing_Information_Protocol) and [OSPF](https://en.wikipedia.org/wiki/Open_Shortest_Path_First) and specifically designed for dynamic networks [AODV](https://en.wikipedia.org/wiki/Ad_hoc_On-Demand_Distance_Vector_Routing), [DSR](https://en.wikipedia.org/wiki/Dynamic_Source_Routing), [OLSR](https://en.wikipedia.org/wiki/Optimized_Link_State_Routing_Protocol). I reviewed them, how they work and decided that most interesting will be implement something that look like AODV. So during implementation I didn't read [RFC 3561](https://datatracker.ietf.org/doc/rfc3561/) because I wanted to came up with my own solution without understanding of what happening under the hood of AODV and I believe I did well.   

### User-space

Mesh network created by the app it's a logical mesh network. So transportation of packages trusted to UDP/IP stack, this means that all nodes on the same network can communicate without any help from my application. To solve this problem I decided to support multiple interfaces, this forces app to route packages between networks and allows to create something that look like real mesh network.
