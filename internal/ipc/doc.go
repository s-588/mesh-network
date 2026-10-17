// Package ipc talks to a running mesh-node over its localhost HTTP IPC API.
// The CLI and the TUI both use this client. Wire types and payload checks
// live here so the IPC server and its callers share one contract.
package ipc
