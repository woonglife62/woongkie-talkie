package handler

import (
	"sync"

	redisclient "github.com/woonglife62/woongkie-talkie/pkg/redis"
)

// roomManager manages all room hubs
type roomManager struct {
	hubs   map[string]*Hub
	mu     sync.RWMutex
	broker *redisclient.Broker
}

// RoomMgr is the global room manager instance
var RoomMgr = &roomManager{
	hubs: make(map[string]*Hub),
}

// SetBroker sets the Redis Pub/Sub broker on the room manager.
// New hubs created after this call will use the broker.
func (rm *roomManager) SetBroker(broker *redisclient.Broker) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.broker = broker
}

func (rm *roomManager) GetOrCreateHub(roomID string) *Hub {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if hub, ok := rm.hubs[roomID]; ok {
		return hub
	}
	hub := NewHub(roomID, rm.broker)
	rm.hubs[roomID] = hub
	go hub.Run()
	return hub
}

func (rm *roomManager) GetHub(roomID string) *Hub {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.hubs[roomID]
}

// RemoveHub closes all connections and removes the hub for a room
func (rm *roomManager) RemoveHub(roomID string) {
	rm.mu.Lock()
	hub, ok := rm.hubs[roomID]
	if ok {
		delete(rm.hubs, roomID)
	}
	rm.mu.Unlock()
	if hub != nil {
		close(hub.stop)
		// closeAllClients is called inside hub.Run on stop signal.
	}
}

func (rm *roomManager) ShutdownAll() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	for id, hub := range rm.hubs {
		close(hub.stop)
		// closeAllClients is called inside hub.Run on stop signal.
		delete(rm.hubs, id)
	}
	if rm.broker != nil {
		rm.broker.Close()
	}
}

// GetOnlineMembers returns online usernames for a room
func (rm *roomManager) GetOnlineMembers(roomID string) []string {
	hub := rm.GetHub(roomID)
	if hub == nil {
		return []string{}
	}
	return hub.GetMemberNames()
}
