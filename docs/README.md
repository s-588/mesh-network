[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://www.docker.com/)
[![Bubble Tea](https://img.shields.io/badge/Bubble_Tea-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://github.com/charmbracelet/bubbletea)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge)](LICENSE)

# Mesh Network – AODV Mesh Node

A Go implementation of an AODV-style ad-hoc routing node with a Bubble Tea TUI and CLI. Useful for learning reactive mesh routing and simulating multi-node topologies with Docker.

## Key Features

- Full AODV-like mechanics: RREQ, RREP, RERR, periodic HELLO
- On-demand route discovery with sequence numbers and precursor lists
- Multi-interface support
- Interactive Bubble Tea TUI
- basic CLI
- Ready-to-run Docker Compose multi-node simulation

## Built With

- Go 1.26+
- Bubble Tea + Bubbles + Lip Gloss (TUI)
- urfave/cli, godotenv, snowflake

## Getting Started

### Prerequisites

- Go 1.26+
- Docker & Docker Compose (for the multi-node simulation)

### Quick demonstration of work

```bash
git clone https://github.com/s-588/mesh-network.git
cd mesh-network
docker compose up 
```

This starts 11 nodes across four networks. Attach to a node:

```bash
docker compose exec -it node2 /usr/local/bin/mesh-node
# or
docker compose attach node2
```

<img width="1100" height="571" alt="1-main_functional" src="https://github.com/user-attachments/assets/398301f7-5336-4fb1-b4e7-cf4bbd0dbe21" />

Network topology of this simulation:

![Docker Compose topology](https://github.com/user-attachments/assets/63381571-041b-4c04-8fc7-bcbc5f2f0e75)

| Network | Nodes | Also connected to |
| --------- | ------- | ------------------- |
| net-a `172.20.1.0/24` | 1, 2, 3, 4, 9 | node1 → net-b, node9 → net-c |
| net-b `172.20.2.0/24` | 1, 5, 6, 7 | node7 → net-c |
| net-c `172.20.3.0/24` | 7, 8, 9, 10 | node10 → net-d |
| net-d `172.20.4.0/24` | 10, 11 | — |

Nodes 1, 7, 9, and 10 are the gateways (`INTERFACE=eth0,eth1`). Everyone else has a single interface. Node 2 and node 11 do not share a LAN, so a message between them has to be routed.

### Install / Build Locally

```bash
go install github.com/s-588/mesh-network/cmd/mesh-node@latest
# or
go build -o mesh-node ./cmd/mesh-node
```

## Usage

### TUI

```bash
mesh-node
```

Keyboard: `Tab` / `Shift+Tab` to move focus, `Enter` to advance a field or send, `Esc` / `Ctrl+C` to quit.

Left side is the destination id and the message. Right side is logs. Bottom is the neighbours table, then the routes table. The header shows this node's id and interfaces.

<img width="1100" height="571" alt="3-tui" src="https://github.com/user-attachments/assets/99afd3e3-766f-4363-a82c-4028a4f7638a" />

**Send a message**

Focus order is destination → message → Send. Type a live neighbour id, press Enter, type the message, press Enter on Send. 

<img width="1100" height="571" alt="4-send" src="https://github.com/user-attachments/assets/9f175b4e-03fa-4f0f-b3f7-88f0179de62a" />

**Neighbours & routes**

Tab into the neighbours table (populated by HELLO), then into the routes table. Routes appear after a message or RREQ to a non-neighbour.

### CLI

All `send` / `show` / `kill` commands require a running node.

```bash
mesh-node help

# Start a background node
mesh-node --daemon --id 1 --interface eth0 --log-level DEBUG

# Send
mesh-node send msg <target_id> "hello"
# If you want to find route to Node without sending anything
mesh-node send rreq <target_id>

# Inspect
mesh-node show messages      # aliases: m, msgs
mesh-node show neighbours    # aliases: n, neigh
mesh-node show routes        # alias: r
mesh-node show config        # aliases: c, cfg

# Stop background Node
mesh-node kill               # alias: stop
```

## Configuration

| Env / Flag | Description | Default |
| ------------ | ------------- | --------- |
| `ID` / `--id` | Node ID | auto (Snowflake) |
| `PORT` / `--port` | UDP port for mesh traffic | 6040 |
| `INTERFACE` / `--interface`, `-i` | Comma-separated interfaces | `eth0` |
| `TTL` / `--ttl` | Max hops | 20 |
| `LIFETIME` / `--lifetime` | Route and cached-message lifetime (seconds) | 30 |
| `HELLO_INTERVAL` / `--hello-interval` | HELLO interval (seconds, 1–86400) | 5 |
| `LOG_LEVEL` / `--log-level` | `DEBUG` / `INFO` / `WARN` / `ERROR` | `INFO` |
| `LOG_FILE` / `--log-file` | Log file path. Empty = stderr | (none) |
| `DAEMON` / `--daemon`, `--server`, `-d` | Background server, no TUI | false |

IPC for the CLI and TUI is always `127.0.0.1:6242`. This is separate from the mesh UDP port.

## Internals

This project started as a way to learn how mesh networks work in practice. After looking at classic protocols (RIP, OSPF) and mesh-oriented ones (AODV, DSR, OLSR), I decided to implement something AODV-like. I deliberately did **not** develop logic according to [RFC 3561](https://datatracker.ietf.org/doc/rfc3561/) because the goal was to invent a working solution from basic principles rather than copy the standard.

Nodes communicate over UDP on one or more network interfaces, using broadcast for discovery and unicast for data and replies. Each node has a unique 64-bit ID. Messages share a common header followed by type-specific fields. Five message types are defined: HELLO for neighbour discovery, RREQ (route request), RREP (route reply), RERR (route error), and DATA.

Neighbour discovery and maintenance work continuously. Every node periodically broadcasts HELLO messages on all configured interfaces, advertising its ID and listening port. Receivers update a thread-safe neighbours table with the sender’s ID, address, interface, and last-seen timestamp. A background collector periodically removes neighbours that have missed several HELLO intervals. When a neighbour dies, any routes that used it as next hop are deleted and RERR messages are sent to registered precursors so upstream nodes can invalidate their own paths.

Routes are discovered on demand. When a node wants to send data to an unknown destination, it queues the payload and floods an RREQ. Intermediate nodes record a reverse route toward the originator, suppress duplicates via the broadcast ID, increment hop count, and rebroadcast if TTL remains. The destination or a node with a sufficiently fresh route replies with a unicast RREP. As the RREP travels back, each hop installs or updates a forward route preferring higher destination sequence numbers or equal sequence with fewer hops and records precursors. Once the originator receives the RREP it drains its pending queue and begins forwarding DATA messages hop-by-hop.

If a DATA packet cannot be forwarded or a next-hop neighbour disappears, an RERR is generated and propagated only to relevant precursors. Routes also expire after their configured lifetime. Sequence numbers and hop counts prevent loops and ensure that fresher or shorter paths replace older ones.

### User-space design

The mesh is a *logical* overlay. Actual packet transport is left to the normal UDP/IP stack, which means nodes on the same L2 network can already reach each other without any help from this program. Supporting multiple interfaces forces the application to route between networks and is what makes the multi-hop mesh behaviour visible and interesting.

## License

MIT
