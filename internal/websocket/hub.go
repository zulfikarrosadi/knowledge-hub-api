package websocket

import (
	"log"
)

// Hub maintains the set of active clients and broadcasts messages to the clients.
// it's the manager to let each connection to communicate to the same websocket
type Hub struct {
	Rooms map[string]map[*Client]bool

	// to send the message to the same websocket connection
	Broadcast chan *Message

	// to add new client / new websocket connection to the map
	Register chan *Client

	// to remove websocket connection from the map
	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan *Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Rooms:      make(map[string]map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			if _, exist := h.Rooms[client.RoomId]; !exist {
				h.Rooms[client.RoomId] = make(map[*Client]bool)
				h.Rooms[client.RoomId][client] = true
				log.Printf("Client registered to room %s", client.RoomId)
			}
		case client := <-h.Unregister:
			if _, exist := h.Rooms[client.RoomId]; exist {
				delete(h.Rooms[client.RoomId], client)
				close(client.Send)
			}
			if len(h.Rooms[client.RoomId]) == 0 {
				delete(h.Rooms, client.RoomId)
			}
			log.Printf("Client unregistered from room %s", client.RoomId)
		case message := <-h.Broadcast:
			if clientInRoom, exist := h.Rooms[message.RoomId]; exist {
				for client := range clientInRoom {
					select {
					case client.Send <- message.Data:
					default:
						close(client.Send)
						delete(clientInRoom, client)
					}
				}
			}
		}
	}
}
