package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

func TestMain(m *testing.M) {
	logger.Initialize(true)
	os.Exit(m.Run())
}

// dialWS opens a WebSocket connection to the given httptest.Server URL path.
func dialWS(t *testing.T, server *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(server.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("failed to dial WebSocket: %v", err)
	}
	return conn
}

// newTestServer starts an httptest.Server that upgrades HTTP to WebSocket and
// runs the provided handler function for each connection.
func newTestServer(t *testing.T, fn func(conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		fn(conn)
	}))
}

// makeClient creates a *Client backed by a real WebSocket connection pair
// (server side + client side). The returned cleanup func closes both ends.
// The client's writePump is started so hub.Broadcast messages flow through
// client.Send -> writePump -> WebSocket -> clientConn.
func makeClient(t *testing.T, username, roomID string) (*Client, *websocket.Conn, func()) {
	t.Helper()
	var serverConn *websocket.Conn
	var mu sync.Mutex
	ready := make(chan struct{})

	srv := newTestServer(t, func(conn *websocket.Conn) {
		mu.Lock()
		serverConn = conn
		mu.Unlock()
		close(ready)
		// keep the handler alive until the test is done
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	clientConn := dialWS(t, srv, "/")
	<-ready

	mu.Lock()
	sc := serverConn
	mu.Unlock()

	client := &Client{
		Conn:     sc,
		Send:     make(chan mongodb.ChatMessage, 256),
		Username: username,
		RoomID:   roomID,
	}
	// Start writePump so messages sent to client.Send reach clientConn.
	go client.writePump()

	cleanup := func() {
		clientConn.Close()
		sc.Close()
		srv.Close()
	}
	return client, clientConn, cleanup
}

// -------------------------------------------------------------------
// Hub lifecycle
// -------------------------------------------------------------------

func TestNewHub(t *testing.T) {
	hub := NewHub("room-1", nil)
	if hub.RoomID != "room-1" {
		t.Errorf("expected RoomID=room-1, got %s", hub.RoomID)
	}
	if hub.Clients == nil {
		t.Error("Clients map should be initialised")
	}
	if hub.Broadcast == nil {
		t.Error("Broadcast channel should be initialised")
	}
	if hub.Register == nil {
		t.Error("Register channel should be initialised")
	}
	if hub.Unregister == nil {
		t.Error("Unregister channel should be initialised")
	}
	if hub.stop == nil {
		t.Error("stop channel should be initialised")
	}
}

// -------------------------------------------------------------------
// Hub.Register
// -------------------------------------------------------------------

func TestHub_RegisterClient(t *testing.T) {
	hub := NewHub("room-reg", nil)
	go hub.Run()
	defer close(hub.stop)

	client, _, cleanup := makeClient(t, "alice", "room-reg")
	defer cleanup()

	hub.Register <- client
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	_, ok := hub.Clients[client]
	hub.mu.RUnlock()

	if !ok {
		t.Error("client should be in hub.Clients after Register")
	}
}

// -------------------------------------------------------------------
// Hub.Unregister
// -------------------------------------------------------------------

func TestHub_UnregisterClient(t *testing.T) {
	hub := NewHub("room-unreg", nil)
	go hub.Run()
	defer close(hub.stop)

	client, _, cleanup := makeClient(t, "bob", "room-unreg")
	defer cleanup()

	hub.Register <- client
	time.Sleep(50 * time.Millisecond)

	hub.Unregister <- client
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	_, ok := hub.Clients[client]
	hub.mu.RUnlock()

	if ok {
		t.Error("client should be removed from hub.Clients after Unregister")
	}
}

// -------------------------------------------------------------------
// Hub.Broadcast – all clients receive the message
// -------------------------------------------------------------------

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub("room-bcast", nil)
	go hub.Run()
	defer close(hub.stop)

	client1, wire1, cleanup1 := makeClient(t, "user1", "room-bcast")
	defer cleanup1()
	client2, wire2, cleanup2 := makeClient(t, "user2", "room-bcast")
	defer cleanup2()

	hub.Register <- client1
	hub.Register <- client2
	time.Sleep(50 * time.Millisecond)

	msg := mongodb.ChatMessage{
		User:    "user1",
		Message: "hello",
		RoomID:  "room-bcast",
		Event:   "MSG",
	}
	hub.Broadcast <- msg
	time.Sleep(100 * time.Millisecond)

	// Both wire connections should receive a message.
	wire1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var recv1 mongodb.ChatMessage
	if err := wire1.ReadJSON(&recv1); err != nil {
		t.Errorf("user1 (wire1) did not receive broadcast: %v", err)
	}

	wire2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var recv2 mongodb.ChatMessage
	if err := wire2.ReadJSON(&recv2); err != nil {
		t.Errorf("user2 (wire2) did not receive broadcast: %v", err)
	}
}

// -------------------------------------------------------------------
// Hub.Broadcast – Owner flag
// -------------------------------------------------------------------

func TestHub_BroadcastOwnerFlag(t *testing.T) {
	hub := NewHub("room-owner", nil)
	go hub.Run()
	defer close(hub.stop)

	// sender
	sender, wireSender, cleanupS := makeClient(t, "sender", "room-owner")
	defer cleanupS()
	// observer
	observer, wireObserver, cleanupO := makeClient(t, "observer", "room-owner")
	defer cleanupO()

	hub.Register <- sender
	hub.Register <- observer
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast <- mongodb.ChatMessage{
		User:    "sender",
		Message: "hi",
		RoomID:  "room-owner",
		Event:   "MSG",
	}
	time.Sleep(100 * time.Millisecond)

	// Read the broadcast message from each wire, skipping PRESENCE events.
	fromSenderWire, ok := readNonPresence(t, wireSender, time.Second)
	if !ok {
		t.Fatal("sender wire: expected to receive broadcast")
	}
	fromObserverWire, ok := readNonPresence(t, wireObserver, time.Second)
	if !ok {
		t.Fatal("observer wire: expected to receive broadcast")
	}

	if !fromSenderWire.Owner {
		t.Error("message delivered to sender should have Owner=true")
	}
	if fromObserverWire.Owner {
		t.Error("message delivered to observer should have Owner=false")
	}
}

// -------------------------------------------------------------------
// Hub.Broadcast – OPEN event message format
// -------------------------------------------------------------------

func TestHub_BroadcastOpenEvent(t *testing.T) {
	hub := NewHub("room-open", nil)
	go hub.Run()
	defer close(hub.stop)

	client, wire, cleanup := makeClient(t, "alice", "room-open")
	defer cleanup()

	hub.Register <- client
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast <- mongodb.ChatMessage{
		User:   "alice",
		RoomID: "room-open",
		Event:  "OPEN",
	}
	time.Sleep(100 * time.Millisecond)

	recv, ok := readNonPresence(t, wire, time.Second)
	if !ok {
		t.Fatal("failed to read OPEN message")
	}

	want := "---- alice님이 입장하셨습니다. ----"
	if recv.Message != want {
		t.Errorf("OPEN event message: got %q, want %q", recv.Message, want)
	}
}

// -------------------------------------------------------------------
// Hub.Broadcast – CLOSE event message format
// -------------------------------------------------------------------

func TestHub_BroadcastCloseEvent(t *testing.T) {
	hub := NewHub("room-close", nil)
	go hub.Run()
	defer close(hub.stop)

	client, wire, cleanup := makeClient(t, "bob", "room-close")
	defer cleanup()

	hub.Register <- client
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast <- mongodb.ChatMessage{
		User:   "bob",
		RoomID: "room-close",
		Event:  "CLOSE",
	}
	time.Sleep(100 * time.Millisecond)

	recv, ok := readNonPresence(t, wire, time.Second)
	if !ok {
		t.Fatal("failed to read CLOSE message")
	}

	want := "---- bob님이 퇴장하셨습니다. ----"
	if recv.Message != want {
		t.Errorf("CLOSE event message: got %q, want %q", recv.Message, want)
	}
}

// -------------------------------------------------------------------
// RoomManager
// -------------------------------------------------------------------

// newRoomManager returns a fresh, isolated roomManager for testing
// so we do not pollute the global RoomMgr.
func newRoomManager() *roomManager {
	return &roomManager{hubs: make(map[string]*Hub)}
}

func TestRoomManager_GetOrCreateHub(t *testing.T) {
	rm := newRoomManager()

	hub := rm.GetOrCreateHub("r1")
	if hub == nil {
		t.Fatal("GetOrCreateHub returned nil")
	}
	if hub.RoomID != "r1" {
		t.Errorf("expected RoomID=r1, got %s", hub.RoomID)
	}
	// Verify it is stored.
	rm.mu.RLock()
	stored, ok := rm.hubs["r1"]
	rm.mu.RUnlock()
	if !ok || stored != hub {
		t.Error("hub should be stored in the hubs map")
	}
	// Stop the hub goroutine started by GetOrCreateHub.
	close(hub.stop)
}

func TestRoomManager_GetOrCreateHub_Idempotent(t *testing.T) {
	rm := newRoomManager()

	hub1 := rm.GetOrCreateHub("r2")
	hub2 := rm.GetOrCreateHub("r2")
	if hub1 != hub2 {
		t.Error("GetOrCreateHub should return the same hub on second call")
	}
	close(hub1.stop)
}

func TestRoomManager_GetHub(t *testing.T) {
	rm := newRoomManager()

	// Non-existing hub returns nil.
	if got := rm.GetHub("nonexistent"); got != nil {
		t.Errorf("expected nil for nonexistent hub, got %v", got)
	}

	// After creation it is retrievable.
	hub := rm.GetOrCreateHub("r3")
	if got := rm.GetHub("r3"); got != hub {
		t.Error("GetHub should return the hub that was created")
	}
	close(hub.stop)
}

func TestRoomManager_RemoveHub(t *testing.T) {
	rm := newRoomManager()

	_ = rm.GetOrCreateHub("r4")
	// RemoveHub closes hub.stop internally, so we must NOT close it again.
	rm.RemoveHub("r4")

	if got := rm.GetHub("r4"); got != nil {
		t.Error("hub should be gone after RemoveHub")
	}
}

func TestRoomManager_ShutdownAll(t *testing.T) {
	rm := newRoomManager()

	rm.GetOrCreateHub("sa1")
	rm.GetOrCreateHub("sa2")
	rm.GetOrCreateHub("sa3")

	rm.ShutdownAll()

	rm.mu.RLock()
	count := len(rm.hubs)
	rm.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 hubs after ShutdownAll, got %d", count)
	}
}

func TestRoomManager_GetOnlineMembers_NoHub(t *testing.T) {
	rm := newRoomManager()
	members := rm.GetOnlineMembers("does-not-exist")
	if members == nil || len(members) != 0 {
		t.Errorf("expected empty slice for missing room, got %v", members)
	}
}

// -------------------------------------------------------------------
// Race condition: concurrent Register/Unregister
// -------------------------------------------------------------------

func TestHub_ConcurrentRegisterUnregister(t *testing.T) {
	hub := NewHub("room-race-reg", nil)
	go hub.Run()
	defer close(hub.stop)

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			username := fmt.Sprintf("user%d", idx)
			client, _, cleanup := makeClient(t, username, "room-race-reg")
			defer cleanup()

			hub.Register <- client
			time.Sleep(10 * time.Millisecond)
			hub.Unregister <- client
		}(i)
	}

	wg.Wait()
	// Allow hub to drain all pending register/unregister operations.
	time.Sleep(100 * time.Millisecond)

	hub.mu.RLock()
	count := len(hub.Clients)
	hub.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 clients after all goroutines finish, got %d", count)
	}
}

func TestRoomManager_GetOnlineMembers(t *testing.T) {
	rm := newRoomManager()
	hub := rm.GetOrCreateHub("online-room")
	defer rm.RemoveHub("online-room")

	c1, _, cl1 := makeClient(t, "alice", "online-room")
	defer cl1()
	c2, _, cl2 := makeClient(t, "bob", "online-room")
	defer cl2()

	hub.Register <- c1
	hub.Register <- c2
	time.Sleep(50 * time.Millisecond)

	members := rm.GetOnlineMembers("online-room")
	if len(members) != 2 {
		t.Errorf("expected 2 online members, got %d: %v", len(members), members)
	}
	names := map[string]bool{}
	for _, n := range members {
		names[n] = true
	}
	if !names["alice"] || !names["bob"] {
		t.Errorf("expected alice and bob, got %v", members)
	}
}
