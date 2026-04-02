package qmp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Client is a QMP client that communicates with QEMU over a Unix socket.
type Client struct {
	socketPath  string
	balloonPath string
	conn        net.Conn
	reader      *bufio.Reader
	mu          sync.Mutex
}

const ioTimeout = 10 * time.Second

// NewClient creates a new QMP client for the given socket and balloon device path.
func NewClient(socketPath, balloonPath string) *Client {
	return &Client{socketPath: socketPath, balloonPath: balloonPath}
}

// Connect establishes a connection to the QEMU Unix socket via QMP.
func (c *Client) Connect() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return errors.New("already connected")
	}

	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("connecting to QEMU socket %s: %w", c.socketPath, err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)

	defer func() {
		if err != nil {
			c.conn.Close()
			c.conn = nil
			c.reader = nil
		}
	}()

	c.conn.SetDeadline(time.Now().Add(ioTimeout))
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("reading QMP greeting: %w", err)
	}

	var g greeting
	if err := json.Unmarshal(line, &g); err != nil {
		return fmt.Errorf("parsing QMP greeting: %w", err)
	}
	if g.QMP == nil {
		return errors.New("invalid QMP greeting: missing QMP field")
	}

	if _, err := c.call("qmp_capabilities", nil); err != nil {
		return fmt.Errorf("negotiating QMP capabilities: %w", err)
	}

	return nil
}

// Close closes the connection to the QEMU socket.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.reader = nil
		return err
	}
	return nil
}

func (c *Client) call(cmd string, args any) (json.RawMessage, error) {
	if c.conn == nil {
		return nil, errors.New("not connected")
	}

	msg := command{Execute: cmd}
	if args != nil {
		argBytes, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("marshaling arguments: %w", err)
		}
		msg.Arguments = argBytes
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshaling command: %w", err)
	}

	c.conn.SetDeadline(time.Now().Add(ioTimeout))

	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("writing command: %w", err)
	}

	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		var resp response
		if err := json.Unmarshal(line, &resp); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}

		if resp.Event != "" {
			continue
		}

		if resp.Error != nil {
			return nil, resp.Error
		}

		return resp.Return, nil
	}
}

// GetBalloonTarget returns the current balloon target in bytes.
func (c *Client) GetBalloonTarget() (BalloonInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	raw, err := c.call("query-balloon", nil)
	if err != nil {
		return BalloonInfo{}, err
	}
	var info BalloonInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return BalloonInfo{}, fmt.Errorf("parsing balloon info: %w", err)
	}
	return info, nil
}

// SetBalloonTarget sets the balloon target in bytes.
func (c *Client) SetBalloonTarget(bytes uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.call("balloon", map[string]any{"value": bytes})
	return err
}

// GetBalloonGuestStats returns the guest's balloon memory statistics.
func (c *Client) GetBalloonGuestStats() (GuestStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	raw, err := c.call("qom-get", map[string]any{
		"path":     c.balloonPath,
		"property": "guest-stats",
	})
	if err != nil {
		return GuestStats{}, err
	}
	var stats GuestStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		return GuestStats{}, fmt.Errorf("parsing guest stats: %w", err)
	}
	return stats, nil
}
