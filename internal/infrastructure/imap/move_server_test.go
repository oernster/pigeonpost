package imap

import (
	"bufio"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// mailboxServer is a scripted IMAP server that keeps real mailbox state, so a move or delete test can
// assert what reached the wire and what survived on the server afterwards. It answers one command at a
// time in the order the client wrote them, which is what exposes a client that pipelines a STORE and an
// EXPUNGE behind a COPY it has not yet seen answered.
type mailboxServer struct {
	// caps is advertised after IMAP4rev1 ("MOVE", "UIDPLUS" or both); empty offers neither.
	caps string
	// copyRefusal, when set, is the text of the tagged NO answering a UID COPY from the
	// refuseCopyFrom'th COPY onwards (counting from one; zero refuses every COPY).
	copyRefusal    string
	refuseCopyFrom int
	// storeRefusal, when set, is the text of the tagged NO answering a UID STORE from the
	// refuseStoreFrom'th STORE onwards (counting from one; zero refuses every STORE).
	storeRefusal    string
	refuseStoreFrom int

	commands commandLog
	mu       sync.Mutex
	boxes    map[string][]*storedMessage
	nextUID  map[string]uint32
	copies   int
	stores   int
}

// storedMessage is one message on the scripted server: its UID and whether \Deleted is set on it.
type storedMessage struct {
	uid     uint32
	deleted bool
}

// uidValidity is the one UIDVALIDITY every scripted mailbox reports.
const uidValidity = 1

// newMailboxServer seeds the named mailbox with the given UIDs and starts serving on a loopback port.
func newMailboxServer(t *testing.T, srv *mailboxServer, box string, uids ...uint32) (host string, port int) {
	t.Helper()
	srv.boxes = map[string][]*storedMessage{}
	srv.nextUID = map[string]uint32{}
	for _, u := range uids {
		srv.boxes[box] = append(srv.boxes[box], &storedMessage{uid: u})
		srv.nextUID[box] = max(srv.nextUID[box], u+1)
	}
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
			go srv.serve(conn)
		}
	}()
	addr := listener.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

// flagDeleted marks a seeded message \Deleted, as another client in "mark as deleted" mode leaves it.
func (srv *mailboxServer) flagDeleted(box string, uid uint32) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	for _, m := range srv.boxes[box] {
		if m.uid == uid {
			m.deleted = true
		}
	}
}

// uids answers the UIDs a mailbox holds now, in order.
func (srv *mailboxServer) uids(box string) []uint32 {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	out := make([]uint32, 0, len(srv.boxes[box]))
	for _, m := range srv.boxes[box] {
		out = append(out, m.uid)
	}
	return out
}

func (srv *mailboxServer) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	write := func(lines ...string) {
		for _, line := range lines {
			_, _ = writer.WriteString(line + crlf)
		}
		_ = writer.Flush()
	}
	caps := strings.TrimSpace("IMAP4rev1 " + srv.caps)
	write("* OK [CAPABILITY " + caps + "] ready")
	selected := ""
	for {
		raw, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line := strings.TrimRight(raw, crlf)
		srv.commands.add(line)
		args := strings.Fields(line)
		tag, verb := args[0], strings.ToUpper(args[1])
		if verb == "UID" {
			verb, args = "UID "+strings.ToUpper(args[2]), args[1:]
		}
		switch verb {
		case "CAPABILITY":
			write("* CAPABILITY "+caps, tag+" OK done")
		case "SELECT":
			selected = unquote(args[2])
			write(srv.selectReply(selected, tag)...)
		case "UID COPY", "UID MOVE":
			write(srv.copy(selected, args[2], unquote(args[3]), verb == "UID MOVE", tag)...)
		case "UID STORE":
			write(srv.store(selected, args[2], tag))
		case "EXPUNGE":
			write(srv.expunge(selected, nil, tag)...)
		case "UID EXPUNGE":
			set := parseTestUIDSet(args[2])
			write(srv.expunge(selected, set, tag)...)
		case "UID SEARCH":
			write("* SEARCH"+srv.uidList(selected), tag+" OK done")
		case "LOGOUT":
			write("* BYE", tag+" OK done")
			return
		default:
			write(tag + " OK done")
		}
	}
}

func (srv *mailboxServer) selectReply(box, tag string) []string {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return []string{
		fmt.Sprintf("* %d EXISTS", len(srv.boxes[box])),
		fmt.Sprintf("* OK [UIDVALIDITY %d] ok", uidValidity),
		fmt.Sprintf("* OK [UIDNEXT %d] ok", max(srv.nextUID[box], 1)),
		tag + " OK [READ-WRITE] selected",
	}
}

// copy duplicates the matching messages into dest (removing them from box too when move is set) and
// answers COPYUID when UIDPLUS is advertised; a scripted refusal answers NO instead.
func (srv *mailboxServer) copy(box, setArg, dest string, move bool, tag string) []string {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.copies++
	if srv.copyRefusal != "" && srv.copies >= srv.refuseCopyFrom {
		return []string{tag + " NO " + srv.copyRefusal}
	}
	set := parseTestUIDSet(setArg)
	var src, dst []string
	for _, m := range srv.boxes[box] {
		if !set[m.uid] {
			continue
		}
		next := max(srv.nextUID[dest], 1)
		srv.nextUID[dest] = next + 1
		srv.boxes[dest] = append(srv.boxes[dest], &storedMessage{uid: next})
		src = append(src, strconv.FormatUint(uint64(m.uid), 10))
		dst = append(dst, strconv.FormatUint(uint64(next), 10))
	}
	var lines []string
	if move {
		lines = srv.removeLocked(box, func(m *storedMessage) bool { return set[m.uid] })
	}
	code := ""
	if strings.Contains(srv.caps, "UIDPLUS") && len(src) > 0 {
		code = fmt.Sprintf("[COPYUID %d %s %s] ", uidValidity, strings.Join(src, ","), strings.Join(dst, ","))
	}
	return append(lines, tag+" OK "+code+"done")
}

func (srv *mailboxServer) store(box, setArg, tag string) string {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.stores++
	if srv.storeRefusal != "" && srv.stores >= srv.refuseStoreFrom {
		return tag + " NO " + srv.storeRefusal
	}
	set := parseTestUIDSet(setArg)
	for _, m := range srv.boxes[box] {
		if set[m.uid] {
			m.deleted = true
		}
	}
	return tag + " OK done"
}

// expunge removes the \Deleted messages of box, only those within set when set is not nil.
func (srv *mailboxServer) expunge(box string, set map[uint32]bool, tag string) []string {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	lines := srv.removeLocked(box, func(m *storedMessage) bool { return m.deleted && (set == nil || set[m.uid]) })
	return append(lines, tag+" OK done")
}

// removeLocked drops the messages gone reports, answering one EXPUNGE line per message with the
// sequence number it held at the moment it went.
func (srv *mailboxServer) removeLocked(box string, gone func(*storedMessage) bool) []string {
	var lines []string
	kept := srv.boxes[box][:0]
	for _, m := range srv.boxes[box] {
		if gone(m) {
			lines = append(lines, fmt.Sprintf("* %d EXPUNGE", len(kept)+1))
			continue
		}
		kept = append(kept, m)
	}
	srv.boxes[box] = kept
	return lines
}

func (srv *mailboxServer) uidList(box string) string {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	var b strings.Builder
	for _, m := range srv.boxes[box] {
		b.WriteString(" " + strconv.FormatUint(uint64(m.uid), 10))
	}
	return b.String()
}

// parseTestUIDSet reads a UID set argument ("7", "1:500", "7,9:10") into a membership map.
func parseTestUIDSet(arg string) map[uint32]bool {
	set := map[uint32]bool{}
	for _, part := range strings.Split(arg, ",") {
		first, last, isRange := strings.Cut(part, ":")
		lo, _ := strconv.ParseUint(first, 10, 32)
		hi := lo
		if isRange {
			hi, _ = strconv.ParseUint(last, 10, 32)
		}
		lo, hi = min(lo, hi), max(lo, hi)
		for u := lo; u <= hi; u++ {
			set[uint32(u)] = true
		}
	}
	return set
}

func unquote(s string) string {
	return strings.Trim(s, `"`)
}

// sortedUIDs answers a copy of uids in ascending order, for comparing mailbox contents.
func sortedUIDs(uids []uint32) []uint32 {
	out := append([]uint32(nil), uids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
