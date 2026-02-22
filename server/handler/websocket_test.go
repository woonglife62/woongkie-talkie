package handler

import (
	"fmt"
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

	registered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		client := newClient(hub, conn, username, hub.RoomID) // uses newClient to init msgLimit (#189)
		hub.Register <- client
		close(registered) // signal that registration was sent (#284)
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

	// Wait for the server-side handler to send the Register message (#284).
	select {
	case <-registered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client to register")
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

// readNonPresence reads the first non-PRESENCE ChatMessage from conn within d duration.
// PRESENCE events are automatically generated when clients register and are skipped.
func readNonPresence(t *testing.T, conn *websocket.Conn, d time.Duration) (mongodb.ChatMessage, bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		conn.SetReadDeadline(deadline)
		var msg mongodb.ChatMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			return msg, false
		}
		if msg.Event == "PRESENCE" {
			continue
		}
		return msg, true
	}
}

// TestWebSocket_HubCommunication verifies that a message sent to hub.Broadcast
// is received by a connected client.
func TestWebSocket_HubCommunication(t *testing.T) {
	hub := NewHub("room-hub-comm", nil)
	go hub.Run()
	defer close(hub.stop)

	conn, teardown := connectTestClient(t, hub, "alice")
	defer teardown()

	want := mongodb.ChatMessage{
		User:    "alice",
		Message: "hello hub",
	}
	hub.Broadcast <- want

	msg, ok := readNonPresence(t, conn, time.Second)
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
			msg, ok := readNonPresence(t, conn, time.Second)
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

	hub.Broadcast <- mongodb.ChatMessage{Event: "OPEN", User: "eve"}

	msg, ok := readNonPresence(t, conn, time.Second)
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

	hub.Broadcast <- mongodb.ChatMessage{Event: "CLOSE", User: "frank"}

	msg, ok := readNonPresence(t, conn, time.Second)
	assert.True(t, ok, "expected message")
	assert.Equal(t, "CLOSE", msg.Event)
	assert.Contains(t, msg.Message, "frank", "leave message should mention username")
	assert.Contains(t, msg.Message, "퇴장", "leave message should contain leave keyword")
}

// TestWebSocket_ConcurrentBroadcast verifies that concurrent broadcasts from
// multiple goroutines are handled without data races.
func TestWebSocket_ConcurrentBroadcast(t *testing.T) {
	hub := NewHub("room-race-bcast", nil)
	go hub.Run()
	defer close(hub.stop)

	const clientCount = 5
	conns := make([]*websocket.Conn, clientCount)
	teardowns := make([]func(), clientCount)
	for i := 0; i < clientCount; i++ {
		username := fmt.Sprintf("racer%d", i)
		conn, teardown := connectTestClient(t, hub, username)
		conns[i] = conn
		teardowns[i] = teardown
	}
	defer func() {
		for _, td := range teardowns {
			td()
		}
	}()

	const broadcasts = 10
	var wg sync.WaitGroup
	wg.Add(clientCount)
	for i := 0; i < clientCount; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < broadcasts; j++ {
				hub.Broadcast <- mongodb.ChatMessage{
					User:    fmt.Sprintf("racer%d", idx),
					Message: fmt.Sprintf("msg-%d-%d", idx, j),
					RoomID:  "room-race-bcast",
					Event:   "MSG",
				}
			}
		}(i)
	}
	wg.Wait()

	// Drain remaining messages from one wire connection to ensure no panic.
	conns[0].SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	for {
		var msg mongodb.ChatMessage
		if err := conns[0].ReadJSON(&msg); err != nil {
			break
		}
	}
}

// TestWebSocket_EmptyMessageRejected verifies that an empty (or whitespace-only)
// message sent by a client is NOT broadcast to other connected clients (#262).
func TestWebSocket_EmptyMessageRejected(t *testing.T) {
	hub := NewHub("room-empty-msg", nil)
	go hub.Run()
	defer close(hub.stop)

	// Sender and an observer.
	connSender, tearSender := connectTestClient(t, hub, "sender")
	defer tearSender()
	connObs, tearObs := connectTestClient(t, hub, "observer")
	defer tearObs()

	// Send an empty message as the sender.
	emptyMsg := mongodb.ChatMessage{
		Event:   "MSG",
		User:    "sender",
		Message: "",
		RoomID:  "room-empty-msg",
	}
	if err := connSender.WriteJSON(emptyMsg); err != nil {
		t.Fatalf("failed to send empty message: %v", err)
	}

	// The observer should NOT receive a MSG event within the timeout.
	// It may receive PRESENCE events which readNonPresence will skip.
	connObs.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	var got mongodb.ChatMessage
	err := connObs.ReadJSON(&got)
	if err == nil && got.Event == "MSG" {
		t.Errorf("observer unexpectedly received an empty MSG: %+v", got)
	}
}

// TestWebSocket_MSG_FILE_Event verifies that MSG_FILE is in the allowedEvents
// whitelist and that a MSG_FILE broadcast is received by connected clients.
func TestWebSocket_MSG_FILE_Event(t *testing.T) {
	// allowedEvents is a package-level var; verify MSG_FILE is present.
	if !allowedEvents["MSG_FILE"] {
		t.Fatal("MSG_FILE must be in the allowedEvents whitelist")
	}

	hub := NewHub("room-msg-file", nil)
	go hub.Run()
	defer close(hub.stop)

	conn, teardown := connectTestClient(t, hub, "uploader")
	defer teardown()

	hub.Broadcast <- mongodb.ChatMessage{
		Event:   "MSG_FILE",
		User:    "uploader",
		Message: "file.png",
		RoomID:  "room-msg-file",
	}

	msg, ok := readNonPresence(t, conn, time.Second)
	assert.True(t, ok, "expected MSG_FILE message to be received")
	assert.Equal(t, "MSG_FILE", msg.Event)
	assert.Equal(t, "uploader", msg.User)
}

// TestWebSocket_TypingEvent verifies that TYPING_START and TYPING_STOP events
// are broadcast to other clients but NOT sent back to the originating user.
func TestWebSocket_TypingEvent(t *testing.T) {
	hub := NewHub("room-typing", nil)
	go hub.Run()
	defer close(hub.stop)

	connTyper, tearTyper := connectTestClient(t, hub, "typer")
	defer tearTyper()
	connObs, tearObs := connectTestClient(t, hub, "observer")
	defer tearObs()

	for _, event := range []string{"TYPING_START", "TYPING_STOP"} {
		t.Run(event, func(t *testing.T) {
			hub.Broadcast <- mongodb.ChatMessage{
				Event:  event,
				User:   "typer",
				RoomID: "room-typing",
			}

			// Observer should receive the typing event.
			connObs.SetReadDeadline(time.Now().Add(time.Second))
			var obsMsg mongodb.ChatMessage
			for {
				if err := connObs.ReadJSON(&obsMsg); err != nil {
					t.Fatalf("observer read error waiting for %s: %v", event, err)
				}
				if obsMsg.Event == "PRESENCE" {
					continue
				}
				break
			}
			assert.Equal(t, event, obsMsg.Event, "observer should receive %s", event)
			assert.Equal(t, "typer", obsMsg.User)

			// Typer should NOT receive their own typing event back.
			connTyper.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			var typerMsg mongodb.ChatMessage
			for {
				err := connTyper.ReadJSON(&typerMsg)
				if err != nil {
					// Timeout is the expected path — no message for the typer.
					break
				}
				if typerMsg.Event == "PRESENCE" {
					continue
				}
				t.Errorf("typer unexpectedly received their own %s event", event)
				break
			}
		})
	}
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
