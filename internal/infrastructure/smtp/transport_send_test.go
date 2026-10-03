package smtp

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// loopbackHost is the only address the test server listens on, so no test here can reach a real network.
const loopbackHost = "127.0.0.1"

// endOfData is the line that closes an SMTP DATA section.
const endOfData = "."

// quitDroppingServer is a hand-written SMTP server on the loopback interface that accepts one message
// in full (250 after DATA) and then drops the connection when the client says QUIT, without the 221
// reply. That is the shape of a server that has taken the mail and gone away: the message is delivered
// and only the goodbye is lost.
type quitDroppingServer struct {
	listener net.Listener
	accepted atomic.Bool
}

func startQuitDroppingServer(t *testing.T) *quitDroppingServer {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort(loopbackHost, "0"))
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}
	server := &quitDroppingServer{listener: listener}
	t.Cleanup(func() { _ = listener.Close() })
	go server.serveOne()
	return server
}

// port is the loopback port the server was given.
func (s *quitDroppingServer) port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

// serveOne answers a single session. Every reply is the minimum a submission client needs; QUIT closes
// the connection unanswered.
func (s *quitDroppingServer) serveOne() {
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)
	reply := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	reply("220 loopback ready")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		verb := strings.ToUpper(strings.Fields(strings.TrimSpace(line) + " ")[0])
		switch verb {
		case "EHLO", "HELO":
			reply("250-loopback")
			reply("250 AUTH PLAIN")
		case "AUTH":
			reply("235 accepted")
		case "MAIL", "RCPT":
			reply("250 ok")
		case "DATA":
			reply("354 go ahead")
			if !s.readData(reader) {
				return
			}
			s.accepted.Store(true)
			reply("250 queued")
		case "QUIT":
			return
		default:
			reply("250 ok")
		}
	}
}

// readData consumes the message body up to its closing dot line, reporting whether it arrived whole.
func (s *quitDroppingServer) readData(reader *bufio.Reader) bool {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		if strings.TrimRight(line, "\r\n") == endOfData {
			return true
		}
	}
}

// staticPassword is a PasswordProvider that always yields the same secret.
type staticPassword struct{}

func (staticPassword) Password(context.Context, domain.Account) (string, error) { return "secret", nil }

// fixedClock is a domain.Clock frozen at one instant.
type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, time.October, 3, 9, 0, 0, 0, time.UTC) }

// loopbackAccount is a password account whose outgoing server is the loopback test server, in plaintext.
func loopbackAccount(t *testing.T, port int) domain.Account {
	t.Helper()
	address, err := domain.NewEmailAddress("Sender", "sender@example.test")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	server, err := domain.NewServerConfig(loopbackHost, port, domain.SecurityNone)
	if err != nil {
		t.Fatalf("server config: %v", err)
	}
	account, err := domain.NewAccount("acct", "Sender", address, domain.ProtocolIMAP, server, server, domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return account
}

// A server that has said 250 to DATA has the message: delivery happened. Losing the connection at QUIT
// afterwards must not turn that into a failure, because a failure invites a resend (a duplicate), skips
// the Sent copy and leaves a replayed item in the Outbox marked failed.
func TestSendSucceedsWhenQuitFailsAfterDataAccepted(t *testing.T) {
	t.Parallel()
	server := startQuitDroppingServer(t)
	account := loopbackAccount(t, server.port())
	to, err := domain.NewEmailAddress("", "rcpt@example.test")
	if err != nil {
		t.Fatalf("recipient: %v", err)
	}
	msg, err := domain.NewOutgoingMessage(domain.OutgoingMessageInput{
		From: account.Address(), To: []domain.EmailAddress{to}, Subject: "hello", Body: "body",
	})
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	transport := NewTransport(staticPassword{}, nil, fixedClock{}, func() string { return "id" })

	sendErr := transport.Send(context.Background(), account, msg)

	if !server.accepted.Load() {
		t.Fatal("the loopback server never accepted the message, so the test proves nothing")
	}
	if sendErr != nil {
		t.Fatalf("Send after an accepted DATA = %v, want nil", sendErr)
	}
}
