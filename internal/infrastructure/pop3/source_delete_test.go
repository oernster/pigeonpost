package pop3

import (
	"context"
	"fmt"
	"net"
	"net/textproto"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// rfc1939Server is a loopback POP3 server on 127.0.0.1 that applies RFC 1939's deletion rules rather than
// a fixed script: DELE only marks a message; RSET unmarks every mark; only an accepted QUIT enters the
// UPDATE state and removes what is marked; a session that ends any other way removes nothing.
type rfc1939Server struct {
	listener    net.Listener
	refuseQuit  bool
	refuseRset  bool
	mu          sync.Mutex
	mailbox     map[int]string // session number to UIDL
	marked      map[int]bool
	committed   []string
	commands    []string
	sessionDone chan struct{}
}

// newRFC1939Server starts the server with the given UIDLs numbered from 1, serving one session.
func newRFC1939Server(t *testing.T, uidls ...string) *rfc1939Server {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	s := &rfc1939Server{listener: listener, mailbox: map[int]string{}, marked: map[int]bool{}, sessionDone: make(chan struct{})}
	for i, uidl := range uidls {
		s.mailbox[i+1] = uidl
	}
	go s.serveOne()
	return s
}

func (s *rfc1939Server) serveOne() {
	defer close(s.sessionDone)
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	tp := textproto.NewConn(conn)
	_ = tp.PrintfLine("+OK ready")
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return // the session ended without QUIT: nothing is removed
		}
		verb, arg, _ := strings.Cut(line, " ")
		s.mu.Lock()
		s.commands = append(s.commands, verb)
		reply, quit := s.handle(verb, arg)
		s.mu.Unlock()
		_ = tp.PrintfLine("%s", reply)
		if quit {
			return
		}
	}
}

// handle applies one command to the mailbox and returns the reply line plus whether the session ends.
func (s *rfc1939Server) handle(verb, arg string) (string, bool) {
	switch verb {
	case "USER", "PASS":
		return "+OK", false
	case "UIDL":
		var b strings.Builder
		b.WriteString("+OK\r\n")
		for _, n := range s.numbers() {
			if !s.marked[n] {
				fmt.Fprintf(&b, "%d %s\r\n", n, s.mailbox[n])
			}
		}
		b.WriteString(".")
		return b.String(), false
	case "DELE":
		n, err := strconv.Atoi(arg)
		if _, ok := s.mailbox[n]; err != nil || !ok || s.marked[n] {
			return "-ERR no such message", false
		}
		s.marked[n] = true
		return "+OK marked", false
	case "RSET":
		if s.refuseRset {
			return "-ERR rset refused", false
		}
		s.marked = map[int]bool{}
		return "+OK", false
	case "QUIT":
		if s.refuseQuit {
			return "-ERR some deleted messages not removed", true
		}
		for _, n := range s.numbers() {
			if s.marked[n] {
				s.committed = append(s.committed, s.mailbox[n])
			}
		}
		return "+OK bye", true
	}
	return "-ERR unknown command", false
}

func (s *rfc1939Server) numbers() []int {
	numbers := make([]int, 0, len(s.mailbox))
	for n := range s.mailbox {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	return numbers
}

// outcome waits for the session to end, then returns what the server removed and the commands it saw.
func (s *rfc1939Server) outcome() ([]string, []string) {
	<-s.sessionDone
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.committed, s.commands
}

// account builds a POP3 account pointed at the loopback server in plain text.
func (s *rfc1939Server) account(t *testing.T) domain.Account {
	t.Helper()
	addr := s.listener.Addr().(*net.TCPAddr)
	in, err := domain.NewServerConfig(addr.IP.String(), addr.Port, domain.SecurityNone)
	if err != nil {
		t.Fatalf("build incoming config: %v", err)
	}
	from, err := domain.NewEmailAddress("", "user@example.com")
	if err != nil {
		t.Fatalf("build address: %v", err)
	}
	account, err := domain.NewAccount("a1", "POP", from, domain.ProtocolPOP3, in, in, domain.AuthPassword)
	if err != nil {
		t.Fatalf("build account: %v", err)
	}
	return account
}

// staticPassword is a hand-written PasswordProvider returning one fixed test secret.
type staticPassword struct{}

func (staticPassword) Password(context.Context, domain.Account) (string, error) { return "test", nil }

func TestDeleteManyCommitsEveryMarkOnQuit(t *testing.T) {
	server := newRFC1939Server(t, "uidl-one", "uidl-two")
	_, err := NewSource(staticPassword{}).DeleteMany(context.Background(), server.account(t), domain.Folder{}, []string{"uidl-one", "uidl-two"}, "")
	if err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}
	if committed, _ := server.outcome(); !reflect.DeepEqual(committed, []string{"uidl-one", "uidl-two"}) {
		t.Errorf("committed = %v, want both removed", committed)
	}
}

func TestDeleteManyWithAMissingUIDLResetsAndCommitsNothing(t *testing.T) {
	server := newRFC1939Server(t, "uidl-one", "uidl-two")
	_, err := NewSource(staticPassword{}).DeleteMany(context.Background(), server.account(t), domain.Folder{}, []string{"uidl-one", "uidl-gone-already"}, "")
	if err == nil {
		t.Fatal("DeleteMany succeeded, want the missing message reported")
	}
	committed, commands := server.outcome()
	if len(committed) != 0 {
		t.Errorf("committed = %v while an error was reported, want nothing removed", committed)
	}
	if !containsCommand(commands, "RSET") {
		t.Errorf("commands = %v, want RSET before QUIT", commands)
	}
}

func TestDeleteReportsARefusedQuit(t *testing.T) {
	server := newRFC1939Server(t, "uidl-one")
	server.refuseQuit = true
	if _, err := NewSource(staticPassword{}).Delete(context.Background(), server.account(t), domain.Folder{}, "uidl-one", ""); err == nil {
		t.Fatal("Delete succeeded although the server refused QUIT and removed nothing")
	}
	if committed, _ := server.outcome(); len(committed) != 0 {
		t.Errorf("committed = %v, want nothing removed", committed)
	}
}

func TestDeleteManyDropsTheSessionWithoutQuitWhenRsetIsRefused(t *testing.T) {
	server := newRFC1939Server(t, "uidl-one")
	server.refuseRset = true
	_, err := NewSource(staticPassword{}).DeleteMany(context.Background(), server.account(t), domain.Folder{}, []string{"uidl-one", "uidl-gone-already"}, "")
	if err == nil || !strings.Contains(err.Error(), "reset") {
		t.Fatalf("err = %v, want the refused RSET named", err)
	}
	committed, commands := server.outcome()
	if len(committed) != 0 || containsCommand(commands, "QUIT") {
		t.Errorf("committed = %v commands = %v, want no QUIT so nothing is removed", committed, commands)
	}
}

func containsCommand(commands []string, verb string) bool {
	for _, c := range commands {
		if c == verb {
			return true
		}
	}
	return false
}
