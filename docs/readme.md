# Development plan

## Phase 1: The "Local Tunnel"
**Goal**: Get two PCs on the same Wi-Fi to "ping" each other through your Go app.
**Tasks**: 
1. Initialize a TUN interface using the water library.
2. Write a Go loop that reads a packet and prints its destination IP.
3. Hardcode the other PC's local IP and send the packet over UDP.

## Phase 2: Local Discovery (mDNS)
**Goal**: Remove the hardcoded IPs for local devices.
**Tasks**: 
1. Integrate an mDNS library (like hashicorp/mdns).
2. Make the app "Announce" itself as a mesh node and "Listen" for others.
3. Update your internal routing table automatically when a peer is found.

## Phase 3: The Bootstrap & NAT Punching (The "Real" Mesh)
**Goal**: Connect a PC at home to a Laptop at college.
**Tasks**: 
1. Build a tiny Go "Rendezvous" server (hosted on a cheap VPS or Heroku).
2. Implement UDP Hole Punching: Both nodes send a packet to the server, the server tells them each other's public ports, and they "punch" through the firewalls.

## Phase 4: Security (The Professional Touch)
**Goal**: Encrypt the traffic so it’s a private LAN.
**Tasks**:
1. Use Go’s crypto/nacl or noise protocol to encrypt the UDP packets.
2. (Optional) Add a simple GUI or CLI to show "Connected Peers."