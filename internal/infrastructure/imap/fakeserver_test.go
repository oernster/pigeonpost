package imap

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// crlf terminates every line of the IMAP wire protocol.
const crlf = "\r\n"

// script is what the fake server answers with. An empty field means the ordinary answer: a login that
// succeeds plus a FETCH response carrying no body structure.
type script struct {
	// bodyStructure is appended to the FETCH response when the client asked for one, so a test can serve
	// a structure the client cannot read.
	bodyStructure string
	// loginRefusal, when set, is the text of a tagged NO answering LOGIN, so a test can drive the
	// sign-in refusal through the real client rather than constructing its error by hand.
	loginRefusal string
	// commands, when set, records every command line the client sends, so a test can assert what went
	// over the wire (how many logins, what a STORE carried).
	commands *commandLog
}

// commandLog collects the command lines a fake server received. The server runs on its own goroutine,
// so the log is guarded.
type commandLog struct {
	mu    sync.Mutex
	lines []string
}

func (c *commandLog) add(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, line)
}

// matching answers the recorded lines that start with the given command, after the tag.
func (c *commandLog) matching(command string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	found := make([]string, 0)
	for _, line := range c.lines {
		if _, rest, ok := strings.Cut(line, " "); ok && strings.HasPrefix(strings.ToUpper(rest), command) {
			found = append(found, rest)
		}
	}
	return found
}

// fakeIMAPServer scripts just enough of an IMAP server to drive the read paths: a greeting, a login, a
// read-only select of one message and a FETCH response.
func fakeIMAPServer(conn net.Conn, s script) {
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	write := func(lines ...string) {
		for _, line := range lines {
			_, _ = writer.WriteString(line + crlf)
		}
		_ = writer.Flush()
	}
	write("* OK [CAPABILITY IMAP4rev1] ready")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		if s.commands != nil {
			s.commands.add(strings.TrimRight(line, crlf))
		}
		fields := strings.SplitN(strings.TrimRight(line, crlf), " ", 3)
		tag, command := fields[0], ""
		if len(fields) > 1 {
			command = strings.ToUpper(fields[1])
		}
		switch command {
		case "CAPABILITY":
			write("* CAPABILITY IMAP4rev1", tag+" OK done")
		case "LOGIN":
			if s.loginRefusal != "" {
				write(tag + " NO " + s.loginRefusal)
				continue
			}
			write(tag + " OK logged in")
		case "SELECT", "EXAMINE":
			write("* 1 EXISTS", "* OK [UIDVALIDITY 1] ok", "* OK [UIDNEXT 2] ok", tag+" OK [READ-ONLY] selected")
		case "FETCH", "UID":
			response := "* 1 FETCH (UID 7 FLAGS (\\Seen) RFC822.SIZE 100 " +
				"ENVELOPE (NIL \"Greetings\" NIL NIL NIL NIL NIL NIL NIL NIL)"
			if strings.Contains(strings.ToUpper(line), "BODYSTRUCTURE") {
				response += " BODYSTRUCTURE " + s.bodyStructure
			}
			write(response+")", tag+" OK done")
		case "LOGOUT":
			write("* BYE", tag+" OK done")
			return
		default:
			write(tag + " OK done")
		}
	}
}

// listenFake starts the scripted server on a loopback port and answers its address, serving one
// connection per accept so a second attempt on a fresh connection is answered too.
func listenFake(t *testing.T, s script) (host string, port int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go fakeIMAPServer(conn, s)
		}
	}()
	addr := listener.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

// fakeAccount builds a password-authenticated account pointed at the scripted server, with no transport
// security so the test needs no certificate.
func fakeAccount(t *testing.T, host string, port int) domain.Account {
	t.Helper()
	address, err := domain.NewEmailAddress("Tester", "tester@example.com")
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	server, err := domain.NewServerConfig(host, port, domain.SecurityNone)
	if err != nil {
		t.Fatalf("server config: %v", err)
	}
	account, err := domain.NewAccount("acct", "Test", address, domain.ProtocolIMAP, server, server, domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return account
}

func fakeFolder(t *testing.T) domain.Folder {
	t.Helper()
	folder, err := domain.NewFolder("folder-1", "acct", "[Google Mail]/All Mail", domain.FolderArchive, 0, 1)
	if err != nil {
		t.Fatalf("folder: %v", err)
	}
	return folder
}

// fakeSource builds the adapter with the stub providers the offline tests use.
func fakeSource() *Source {
	return NewSource(staticPassword{secret: "secret"}, staticToken{token: "token"}, fixedClock{}, func() string { return "id" })
}
