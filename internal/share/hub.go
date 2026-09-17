package share

import (
	"encoding/json"
	"sync"
)

type Client struct {
	RoomID   int64
	MemberID int64
	Send     chan []byte
}

// Hub is an in-process WebSocket fan-out. Persistence remains authoritative;
// reconnecting clients restore missed messages from the ORM-backed history.
type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[*Client]struct{}
}

func NewHub() *Hub { return &Hub{clients: make(map[int64]map[*Client]struct{})} }

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[client.RoomID] == nil {
		h.clients[client.RoomID] = make(map[*Client]struct{})
	}
	h.clients[client.RoomID][client] = struct{}{}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.clients[client.RoomID]
	delete(room, client)
	if len(room) == 0 {
		delete(h.clients, client.RoomID)
	}
}

func (h *Hub) Broadcast(roomID int64, event any) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients[roomID] {
		select {
		case client.Send <- payload:
		default:
			// A slow client will recover from persisted history after reconnect.
		}
	}
}

func (h *Hub) IsOnline(roomID, memberID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients[roomID] {
		if client.MemberID == memberID {
			return true
		}
	}
	return false
}

func (h *Hub) OnlineCount(roomID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[roomID])
}
