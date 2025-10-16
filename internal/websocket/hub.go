package websocket

import (
	"log"
)

// Hub maintains the set of active clients and broadcasts messages to the clients.
// it's the manager to let each connection to communicate to the same websocket
type Hub struct {
	Rooms map[string]map[*Client]bool

	// to send the message to the same websocket connection
	Broadcast chan *WsPayload[WsData]

	// to add new client / new websocket connection to the map
	Register chan *Client

	// to remove websocket connection from the map
	Unregister chan *Client

	CheckRoomId chan *QueryRoomId
}

type QueryRoomId struct {
	RoomId string
	Reply  chan bool
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:   make(chan *WsPayload[WsData]),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		Rooms:       make(map[string]map[*Client]bool),
		CheckRoomId: make(chan *QueryRoomId),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case newClient := <-h.Register:
			client, exist := h.Rooms[newClient.RoomId]
			if !exist {
				h.Rooms[newClient.RoomId] = make(map[*Client]bool)

				res := &WsPayload[WsData]{
					Type:    CREATE_ROOM,
					Status:  "success",
					IsOwner: newClient.IsRoomOwner,
					Data: WsData{
						RoomId: newClient.RoomId,
					},
				}
				newClient.Send <- res
			} else {
				// user is join to existing room
				// broadcasting to room owner
				res := &WsPayload[WsData]{
					Type:    JOIN_ROOM,
					Status:  "pending",
					IsOwner: newClient.IsRoomOwner,
					Data: WsData{
						RoomId: newClient.RoomId,
					},
				}
				for c := range client {
					if c.IsRoomOwner {
						c.Send <- res
						break
					}
				}
			}
			h.Rooms[newClient.RoomId][newClient] = true
			log.Printf("Client registered to room %s", newClient.RoomId)
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
			if clientInRoom, exist := h.Rooms[message.Data.RoomId]; exist {
				for client := range clientInRoom {
					if message.Status == CONN_REJECTED {
						h.Unregister <- client
					} else {
						select {
						case client.Send <- message:
						default:
							close(client.Send)
							delete(clientInRoom, client)
						}
					}
				}
			}
		case query := <-h.CheckRoomId:
			_, exist := h.Rooms[query.RoomId]
			query.Reply <- exist
		}
	}
}
