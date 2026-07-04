package pop3

import (
	"io"
	"net"
	"net/textproto"
	"strings"
	"testing"
)

// fakeServer scripts a minimal POP3 server over a connection, so the client's protocol handling is
// exercised end to end without a real network. greeting is the status line sent on connect.
func fakeServer(conn net.Conn, greeting string) {
	tp := textproto.NewConn(conn)
	defer func() { _ = conn.Close() }()
	_ = tp.PrintfLine("%s", greeting)
	if strings.HasPrefix(greeting, statusErr) {
		return
	}
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		switch {
		case strings.HasPrefix(line, "USER"), strings.HasPrefix(line, "PASS"):
			_ = tp.PrintfLine("+OK")
		case line == "UIDL":
			_ = tp.PrintfLine("+OK")
			writeDot(tp, "1 uidl-one\r\n2 uidl-two\r\n")
		case line == "LIST":
			_ = tp.PrintfLine("+OK")
			writeDot(tp, "1 100\r\n2 200\r\n")
		case strings.HasPrefix(line, "TOP"):
			_ = tp.PrintfLine("+OK")
			writeDot(tp, "Subject: Hi\r\n\r\n")
		case strings.HasPrefix(line, "RETR"):
			_ = tp.PrintfLine("+OK")
			writeDot(tp, "Subject: Hi\r\n\r\nBody line\r\n")
		case strings.HasPrefix(line, "DELE"):
			_ = tp.PrintfLine("+OK marked for deletion")
		case line == "QUIT":
			_ = tp.PrintfLine("+OK bye")
			return
		default:
			_ = tp.PrintfLine("-ERR unknown command")
		}
	}
}

// failAuthServer accepts USER but rejects PASS, to exercise the client's error path.
func failAuthServer(conn net.Conn) {
	tp := textproto.NewConn(conn)
	defer func() { _ = conn.Close() }()
	_ = tp.PrintfLine("+OK ready")
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		switch {
		case strings.HasPrefix(line, "USER"):
			_ = tp.PrintfLine("+OK")
		case strings.HasPrefix(line, "PASS"):
			_ = tp.PrintfLine("-ERR authentication failed")
		default:
			_ = tp.PrintfLine("-ERR unknown command")
		}
	}
}

func writeDot(tp *textproto.Conn, body string) {
	w := tp.DotWriter()
	_, _ = io.WriteString(w, body)
	_ = w.Close()
}

func dialFake(t *testing.T, serve func(net.Conn)) *Client {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	go serve(serverConn)
	client, err := newClient(clientConn)
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	return client
}

func TestClientSession(t *testing.T) {
	client := dialFake(t, func(c net.Conn) { fakeServer(c, "+OK POP3 ready") })
	defer func() { _ = client.Quit() }()

	if err := client.Auth("user", "pass"); err != nil {
		t.Fatalf("Auth: %v", err)
	}

	items, err := client.UIDL()
	if err != nil {
		t.Fatalf("UIDL: %v", err)
	}
	if len(items) != 2 || items[0] != (UIDItem{Number: 1, UID: "uidl-one"}) || items[1] != (UIDItem{Number: 2, UID: "uidl-two"}) {
		t.Fatalf("UIDL items = %+v", items)
	}

	sizes, err := client.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if sizes[1] != 100 || sizes[2] != 200 {
		t.Fatalf("List sizes = %+v", sizes)
	}

	header, err := client.Top(1, topHeaderLines)
	if err != nil {
		t.Fatalf("Top: %v", err)
	}
	if !strings.Contains(string(header), "Subject: Hi") {
		t.Errorf("Top header = %q", header)
	}

	raw, err := client.Retr(1)
	if err != nil {
		t.Fatalf("Retr: %v", err)
	}
	if !strings.Contains(string(raw), "Body line") {
		t.Errorf("Retr body = %q", raw)
	}

	if err := client.Dele(1); err != nil {
		t.Fatalf("Dele: %v", err)
	}
}

func TestClientAuthError(t *testing.T) {
	client := dialFake(t, failAuthServer)
	defer func() { _ = client.Close() }()

	if err := client.Auth("user", "wrong"); err == nil {
		t.Fatal("expected an auth error, got nil")
	}
}

func TestClientGreetingError(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	go fakeServer(serverConn, "-ERR locked")
	if _, err := newClient(clientConn); err == nil {
		t.Fatal("expected a greeting error, got nil")
	}
}

func TestClientServerErrorResponse(t *testing.T) {
	client := dialFake(t, func(c net.Conn) { fakeServer(c, "+OK ready") })
	defer func() { _ = client.Close() }()

	// STAT is not scripted by the fake server, so it replies -ERR, which the client surfaces as an error.
	if err := client.command("STAT"); err == nil {
		t.Fatal("expected an error for an unsupported command, got nil")
	}
}
