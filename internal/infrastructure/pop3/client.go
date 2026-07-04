// Package pop3 implements the application MailSource and AccountVerifier read surface against a live
// POP3 server using a small hand-rolled client (POP3 is a compact line protocol, so it is not worth a
// third-party dependency). The pure mapping between fetched headers and domain summaries lives in
// mapping.go so it is unit-testable without a network, and the client protocol is exercised in
// client_test.go against an in-memory scripted server.
package pop3

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

const (
	statusOK  = "+OK"
	statusErr = "-ERR"
	// uidlFields and listFields are the number of whitespace-separated fields in a UIDL or LIST line
	// (a message number and its value).
	responseFields = 2
	// topHeaderLines is the body-line count passed to TOP to fetch headers only.
	topHeaderLines = 0
)

// Client is a minimal POP3 client over a single connection. It is not safe for concurrent use; each
// mail operation opens its own client.
type Client struct {
	conn net.Conn
	tp   *textproto.Conn
}

// UIDItem pairs a message's session-local number (used by RETR and TOP) with its persistent UIDL (the
// stable handle stored as the message's server id).
type UIDItem struct {
	Number int
	UID    string
}

// dial connects to the account's incoming server with the configured transport security, reads the
// greeting and upgrades with STLS when STARTTLS is selected. A dial failure is wrapped with ErrOffline
// so the caller can treat the server as unreachable.
func dial(incoming domain.ServerConfig) (*Client, error) {
	address := net.JoinHostPort(incoming.Host(), strconv.Itoa(incoming.Port()))

	var (
		conn net.Conn
		err  error
	)
	switch incoming.Security() {
	case domain.SecurityStartTLS, domain.SecurityNone:
		conn, err = net.Dial("tcp", address)
	default:
		conn, err = tls.Dial("tcp", address, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("pop3: dial %s: %w", address, errors.Join(err, domain.ErrOffline))
	}

	client, err := newClient(conn)
	if err != nil {
		return nil, err
	}
	if incoming.Security() == domain.SecurityStartTLS {
		if err := client.startTLS(incoming.Host()); err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	return client, nil
}

// newClient wraps an established connection and consumes the server greeting. It is separate from dial
// so the client can be driven over an in-memory connection in tests.
func newClient(conn net.Conn) (*Client, error) {
	c := &Client{conn: conn, tp: textproto.NewConn(conn)}
	if _, err := c.readStatus(); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// startTLS issues STLS and upgrades the connection to TLS, then re-wraps the textproto reader over the
// encrypted connection.
func (c *Client) startTLS(host string) error {
	if err := c.command("STLS"); err != nil {
		return err
	}
	tlsConn := tls.Client(c.conn, &tls.Config{ServerName: host})
	if err := tlsConn.Handshake(); err != nil {
		return fmt.Errorf("pop3: starttls handshake: %w", err)
	}
	c.conn = tlsConn
	c.tp = textproto.NewConn(tlsConn)
	return nil
}

// readStatus reads a single status line and returns its message on +OK or an error on -ERR.
func (c *Client) readStatus() (string, error) {
	line, err := c.tp.ReadLine()
	if err != nil {
		return "", fmt.Errorf("pop3: read response: %w", err)
	}
	switch {
	case strings.HasPrefix(line, statusOK):
		return strings.TrimSpace(strings.TrimPrefix(line, statusOK)), nil
	case strings.HasPrefix(line, statusErr):
		return "", fmt.Errorf("pop3: server error: %s", strings.TrimSpace(strings.TrimPrefix(line, statusErr)))
	default:
		return "", fmt.Errorf("pop3: unexpected response: %q", line)
	}
}

// command sends a single command and reads its status line, discarding the status message.
func (c *Client) command(format string, args ...any) error {
	if err := c.tp.PrintfLine(format, args...); err != nil {
		return fmt.Errorf("pop3: send command: %w", err)
	}
	_, err := c.readStatus()
	return err
}

// readLines reads a dot-terminated multiline response into its individual lines.
func (c *Client) readLines() ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(c.tp.DotReader())
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("pop3: read multiline: %w", err)
	}
	return lines, nil
}

// readBytes reads a dot-terminated multiline response as raw bytes (a full or partial message).
func (c *Client) readBytes() ([]byte, error) {
	data, err := io.ReadAll(c.tp.DotReader())
	if err != nil {
		return nil, fmt.Errorf("pop3: read message: %w", err)
	}
	return data, nil
}

// Auth authenticates with the USER and PASS commands.
func (c *Client) Auth(user, password string) error {
	if err := c.command("USER %s", user); err != nil {
		return err
	}
	if err := c.command("PASS %s", password); err != nil {
		return err
	}
	return nil
}

// UIDL returns each message's session number paired with its persistent UIDL.
func (c *Client) UIDL() ([]UIDItem, error) {
	if err := c.command("UIDL"); err != nil {
		return nil, err
	}
	lines, err := c.readLines()
	if err != nil {
		return nil, err
	}
	items := make([]UIDItem, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != responseFields {
			continue
		}
		number, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		items = append(items, UIDItem{Number: number, UID: fields[1]})
	}
	return items, nil
}

// List returns the byte size of each message keyed by its session number.
func (c *Client) List() (map[int]int, error) {
	if err := c.command("LIST"); err != nil {
		return nil, err
	}
	lines, err := c.readLines()
	if err != nil {
		return nil, err
	}
	sizes := make(map[int]int, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != responseFields {
			continue
		}
		number, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		size, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		sizes[number] = size
	}
	return sizes, nil
}

// Top fetches a message's headers plus the given number of body lines. With zero body lines it returns
// the headers alone, which is enough to build a summary.
func (c *Client) Top(number, bodyLines int) ([]byte, error) {
	if err := c.command("TOP %d %d", number, bodyLines); err != nil {
		return nil, err
	}
	return c.readBytes()
}

// Retr fetches a message's full raw bytes.
func (c *Client) Retr(number int) ([]byte, error) {
	if err := c.command("RETR %d", number); err != nil {
		return nil, err
	}
	return c.readBytes()
}

// Quit sends QUIT and closes the connection. The QUIT error is ignored because the connection is being
// torn down regardless.
func (c *Client) Quit() error {
	_ = c.command("QUIT")
	return c.Close()
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
