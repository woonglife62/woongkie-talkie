package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

// testUpgrader allows any origin for test servers.
var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// connectTestClient spins up an httptest server that upgrades to WebSocket,
// registers the client with the hub, and reads messages in a loop.
// Returns the client-side *websocket.Conn and the server teardown func.
func connectTestClient(t *testing.T, hub *Hub, username string) (*websocket.Conn, func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		client := &Client{
			Hub:      hub,
			Conn:     conn,
			Send:     make(chan mongodb.ChatMessage, 256),
			Username: username,
			RoomID:   hub.RoomID,
		}
		hub.Register <- client
		go client.writePump()
		// Drain incoming messages (client side drives the test).
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				hub.Unregister <- client
				return
			}
		}
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial error: %v", err)
	}

	teardown := func() {
		conn.Close()
		srv.Close()
	}
	return conn, teardown
}

// readWithTimeout reads a JSON ChatMessage from conn within d duration.
func readWithTimeout(t *testing.T, conn *websocket.Conn, d time.Duration) (mongodb.ChatMessage, bool) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(d))
	var msg mongodb.ChatMessage
	err := conn.ReadJSON(&msg)
	if err != nil {
		return msg, false
	}
	return msg, true
}

// TestWebSocket_HubCommunication verifies that a message sent to hub.Broadcast
// is received by a connected client.
func TestWebSocket_HubCommunication(t *testing.T) {
	hub := NewHub("room-hub-comm", nil)
	go hub.Run()
	defer close(hub.stop)

	conn, teardown := connectTestClient(t, hub, "alice")
	defer teardown()

	// Give the hub time to register the client.
	time.Sleep(20 * time.Millisecond)

	want := mongodb.ChatMessage{
		User:    "alice",
		Message: "hello hub",
	}
	hub.Broadcast <- want

	msg, ok := readWithTimeout(t, conn, time.Second)
	assert.True(t, ok, "expected to receive a message within deadline")
	assert.Equal(t, "alice", msg.User)
	assert.Equal(t, "hello hub", msg.Message)
	assert.True(t, msg.Owner, "sender should be marked as owner")
}

// TestWebSocket_MultipleClients verifies that a broadcast reaches all connected clients.
func TestWebSocket_MultipleClients(t *testing.T) {
	hub := NewHub("room-multi", nil)
	go hub.Run()
	defer close(hub.stop)

	connA, tearA := connectTestClient(t, hub, "alice")
	defer tearA()
	connB, tearB := connectTestClient(t, hub, "bob")
	defer tearB()
	connC, tearC := connectTestClient(t, hub, "carol")
	defer tearC()

	// Allow all three clients to register.
	time.Sleep(30 * time.Millisecond)

	hub.Broadcast <- mongodb.ChatMessage{User: "alice", Message: "group message"}

	var wg sync.WaitGroup
	type result struct {
		msg mongodb.ChatMessage
		ok  bool
	}
	results := make([]result, 3)
	conns := []*websocket.Conn{connA, connB, connC}
	for i, c := range conns {
		wg.Add(1)
		go func(idx int, conn *websocket.Conn) {
			defer wg.Done()
			msg, ok := readWithTimeout(t, conn, time.Second)
			results[idx] = result{msg, ok}
		}(i, c)
	}
	wg.Wait()

	for i, r := range results {
		assert.True(t, r.ok, "client %d should receive the broadcast", i)
		assert.Equal(t, "group message", r.msg.Message)
	}
	// Only the sender (alice, index 0) should be marked Owner.
	assert.True(t, results[0].msg.Owner, "alice should be Owner")
	assert.False(t, results[1].msg.Owner, "bob should not be Owner")
	assert.False(t, results[2].msg.Owner, "carol should not be Owner")
}

// TestWebSocket_ClientDisconnect verifies that after a client disconnects
// it is removed from the hub's client map.
func TestWebSocket_ClientDisconnect(t *testing.T) {
	hub := NewHub("room-disconnect", nil)
	go hub.Run()
	defer close(hub.stop)

	conn, teardown := connectTestClient(t, hub, "dave")
	defer teardown()

	time.Sleep(20 * time.Millisecond)

	hub.mu.RLock()
	countBefore := len(hub.Clients)
	hub.mu.RUnlock()
	assert.Equal(t, 1, countBefore, "hub should have 1 client after connect")

	// Close the client connection — the server-side handler will Unregister.
	conn.Close()

	// Give the hub time to process the Unregister.
	assert.Eventually(t, func() bool {
		hub.mu.RLock()
		defer hub.mu.RUnlock()
		return len(hub.Clients) == 0
	}, time.Second, 10*time.Millisecond, "hub should have 0 clients after disconnect")
}

// TestWebSocket_OpenEvent verifies that an OPEN event results in a join message.
func TestWebSocket_OpenEvent(t *testing.T) {
	hub := NewHub("room-open-event", nil)
	go hub.Run()
	defer close(hub.stop)

	conn, teardown := connectTestClient(t, hub, "eve")
	defer teardown()

	time.Sleep(20 * time.Millisecond)

	hub.Broadcast <- mongodb.ChatMessage{Event: "OPEN", User: "eve"}

	msg, ok := readWithTimeout(t, conn, time.Second)
	assert.True(t, ok, "expected message")
	assert.Equal(t, "OPEN", msg.Event)
	assert.Contains(t, msg.Message, "eve", "join message should mention username")
	assert.Contains(t, msg.Message, "입장", "join message should contain join keyword")
}

// TestWebSocket_CloseEvent verifies that a CLOSE event results in a leave message.
func TestWebSocket_CloseEvent(t *testing.T) {
	hub := NewHub("room-close-event", nil)
	go hub.Run()
	defer close(hub.stop)

	conn, teardown := connectTestClient(t, hub, "frank")
	defer teardown()

	time.Sleep(20 * time.Millisecond)

	hub.Broadcast <- mongodb.ChatMessage{Event: "CLOSE", User: "frank"}

	msg, ok := readWithTimeout(t, conn, time.Second)
	assert.True(t, ok, "expected message")
	assert.Equal(t, "CLOSE", msg.Event)
	assert.Contains(t, msg.Message, "frank", "leave message should mention username")
	assert.Contains(t, msg.Message, "퇴장", "leave message should contain leave keyword")
}

// TestHub_GetMemberNames verifies GetMemberNames returns unique usernames.
func TestHub_GetMemberNames(t *testing.T) {
	hub := NewHub("room-members", nil)
	go hub.Run()
	defer close(hub.stop)

	// Manually register two clients with the same username (two tabs).
	c1 := &Client{Username: "alice", RoomID: hub.RoomID, Send: make(chan mongodb.ChatMessage, 1)}
	c2 := &Client{Username: "alice", RoomID: hub.RoomID, Send: make(chan mongodb.ChatMessage, 1)}
	c3 := &Client{Username: "bob", RoomID: hub.RoomID, Send: make(chan mongodb.ChatMessage, 1)}

	hub.mu.Lock()
	hub.Clients[c1] = true
	hub.Clients[c2] = true
	hub.Clients[c3] = true
	hub.mu.Unlock()

	names := hub.GetMemberNames()
	assert.Len(t, names, 2, "duplicate usernames should be deduplicated")
	assert.ElementsMatch(t, []string{"alice", "bob"}, names)
}
