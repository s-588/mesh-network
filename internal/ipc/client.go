package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// IPCPort is the localhost port of the running node's IPC server.
// The leading colon lets the server concatenate it onto "127.0.0.1".
const IPCPort = ":6242"

var (
	// ipcBase is the IPC server origin. Each request copies it, so concurrent calls are safe.
	ipcBase = url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("127.0.0.1", strings.TrimPrefix(IPCPort, ":")),
	}

	// ErrNodeUnattainable is returned when the IPC server cannot be dialed.
	ErrNodeUnattainable = errors.New("can't connect to background process. Is node started?")
)

// ipcURL builds a request URL for the given path and optional query values.
func ipcURL(path string, query url.Values) string {
	u := ipcBase
	u.Path = path
	if query != nil {
		u.RawQuery = query.Encode()
	}
	return u.String()
}

// doIPC performs one IPC request. A non-200 status is an error whose text is
// the response body, so callers don't treat "invalid destination ID" as success.
func doIPC(ctx context.Context, method, path string, query url.Values) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, method, ipcURL(path, query), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errors.New("timed out talking to the background node")
		}
		return nil, ErrNodeUnattainable
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("can't close response body", "error", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return nil, errors.New(msg)
	}
	return body, nil
}

// IsNodeRunning checks whether the background IPC server is reachable.
func IsNodeRunning(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ipcURL("/neighbours", nil), nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("can't close response body", "error", err)
		}
	}()
	return resp.StatusCode == http.StatusOK
}

// KillNode asks the background node to exit. POST is required so a browser
// prefetch or an accidental GET cannot stop the node.
func KillNode(ctx context.Context) (string, error) {
	body, err := doIPC(ctx, http.MethodPost, "/kill", nil)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// SendRREQ asks the node to discover a path to dst.
func SendRREQ(ctx context.Context, dst uint64) (string, error) {
	q := url.Values{}
	q.Set("dst", strconv.FormatUint(dst, 10))
	body, err := doIPC(ctx, http.MethodGet, "/rreq", q)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// SendMsg sends text to dst via the node's IPC server.
func SendMsg(ctx context.Context, dst uint64, msg string) (string, error) {
	if err := CheckMsg(msg); err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("dst", strconv.FormatUint(dst, 10))
	q.Set("msg", msg) // Query.Get on the server already unescapes; avoid double-escape
	body, err := doIPC(ctx, http.MethodGet, "/send", q)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// GetMessages returns received messages from the node's IPC server.
func GetMessages(ctx context.Context) ([]string, error) {
	body, err := doIPC(ctx, http.MethodGet, "/messages", nil)
	if err != nil {
		return nil, err
	}

	var msgs []string
	if err := json.Unmarshal(body, &msgs); err != nil {
		return nil, fmt.Errorf("can't decode response: %w", err)
	}
	return msgs, nil
}

// GetNeighbours returns structured neighbours from the IPC server.
func GetNeighbours(ctx context.Context) ([]NeighDTO, error) {
	body, err := doIPC(ctx, http.MethodGet, "/neighbours", nil)
	if err != nil {
		return nil, err
	}

	var neighbours []NeighDTO
	if err := json.Unmarshal(body, &neighbours); err != nil {
		return nil, fmt.Errorf("can't decode response: %w", err)
	}
	return neighbours, nil
}

// GetRoutes returns structured routes from the IPC server.
func GetRoutes(ctx context.Context) ([]RouteDTO, error) {
	body, err := doIPC(ctx, http.MethodGet, "/routes", nil)
	if err != nil {
		return nil, err
	}

	var routes []RouteDTO
	if err := json.Unmarshal(body, &routes); err != nil {
		return nil, fmt.Errorf("can't decode response: %w", err)
	}
	return routes, nil
}

// GetConfig returns the structured config from the IPC server.
func GetConfig(ctx context.Context) (ConfigDTO, error) {
	body, err := doIPC(ctx, http.MethodGet, "/config", nil)
	if err != nil {
		return ConfigDTO{}, err
	}

	var cfg ConfigDTO
	if err := json.Unmarshal(body, &cfg); err != nil {
		return ConfigDTO{}, fmt.Errorf("can't decode config response: %w", err)
	}
	return cfg, nil
}
