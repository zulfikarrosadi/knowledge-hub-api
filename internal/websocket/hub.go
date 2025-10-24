package websocket

import (
	"context"
	"log/slog"
)

// Hub maintains the set of active clients and broadcasts messages to the clients.
// it's the manager to let each connection to communicate to the same websocket
type Hub struct {
	Rooms map[string]map[*Client]bool

	// to send the message to the same websocket connection
	Broadcast chan *WsPayload

	// to add new client / new websocket connection to the map
	Register chan *Client

	// to remove websocket connection from the map
	Unregister chan *Client

	CheckRoomId chan *QueryRoomId

	*slog.Logger
}

type QueryRoomId struct {
	RoomId string
	Reply  chan bool
}

func NewHub(slog *slog.Logger) *Hub {
	return &Hub{
		Broadcast:   make(chan *WsPayload),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		Rooms:       make(map[string]map[*Client]bool),
		CheckRoomId: make(chan *QueryRoomId),
		Logger:      slog,
	}
}

func (h *Hub) Run() {
	ctx := context.TODO()

	for {
		select {
		case newClient := <-h.Register:
			client, exist := h.Rooms[newClient.RoomId]
			if !exist {
				h.Rooms[newClient.RoomId] = make(map[*Client]bool)
				h.Logger.LogAttrs(ctx, slog.LevelDebug, "hub_register",
					slog.Group("data",
						slog.String("message", "room not exist, create new one"),
						slog.String("room_id", newClient.RoomId),
						slog.String("username", newClient.Username),
					),
				)
				res := &WsPayload{
					Type:   CREATE_ROOM,
					Status: "success",
					Data:   nil,
					User: User{
						RoomId:   newClient.RoomId,
						Username: newClient.Username,
						IsOwner:  true,
					},
				}
				newClient.Send <- res
			} else {
				// user is join to existing room
				h.Logger.LogAttrs(ctx, slog.LevelDebug, "hub_register",
					slog.Group("data",
						slog.String("message", "room exist, assing user"),
						slog.String("room_id", newClient.RoomId),
						slog.String("username", newClient.Username),
					),
				)
				res := &WsPayload{
					Type:   JOIN_ROOM,
					Status: CONN_PENDING,
					Data:   nil,
					User: User{
						RoomId:   newClient.RoomId,
						Username: newClient.Username,
						IsOwner:  false,
					},
				}

				// send confirmation to new user
				newClient.Send <- res

				// broadcasting to room owner
				for c := range client {
					if c.IsRoomOwner {
						h.Logger.LogAttrs(ctx, slog.LevelDebug, "hub_register",
							slog.Group("data",
								slog.String("message", "broadcast to room owner"),
								slog.String("room_id", newClient.RoomId),
								slog.String("username", newClient.Username),
							),
						)
						c.Send <- res
						break
					}
				}
			}
			h.Rooms[newClient.RoomId][newClient] = true
		case client := <-h.Unregister:
			room, roomExist := h.Rooms[client.RoomId]
			if !roomExist {
				h.Logger.LogAttrs(ctx, slog.LevelWarn, "hub_unregister",
					slog.Group("data",
						slog.String("message", "room not exist, ignoring request"),
						slog.String("room_id", client.RoomId),
						slog.String("username", client.Username),
					),
				)
				continue
			}
			_, clientExist := room[client]
			if !clientExist {
				h.Logger.LogAttrs(ctx, slog.LevelWarn, "hub_unregister",
					slog.Group("data",
						slog.String("message", "client already unregistered, ignoring request"),
						slog.String("room_id", client.RoomId),
						slog.String("username", client.Username),
					),
				)
				continue
			}

			delete(h.Rooms[client.RoomId], client)
			close(client.Send)
			if len(room) == 0 {
				delete(h.Rooms, client.RoomId)
				h.Logger.LogAttrs(ctx, slog.LevelDebug, "hub_unregister",
					slog.Group("data",
						slog.String("message", "room empty and already deleted"),
						slog.String("room_id", client.RoomId),
					),
				)
			}
		case message := <-h.Broadcast:
			clientInRoom, roomExist := h.Rooms[message.User.RoomId]
			if !roomExist {
				continue
			}
			if message.Status == CONN_REJECTED || message.Status == CONN_APPROVED {
				jobs := 2
				for client := range clientInRoom {
					if client.Username == message.User.Username {
						switch message.Status {
						case CONN_REJECTED:
							go func() {
								h.Unregister <- client
							}()
							jobs -= 1
							h.Logger.LogAttrs(ctx, slog.LevelDebug, "hub_broadcast",
								slog.Group("data",
									slog.String("message", "client join rejected"),
									slog.String("room_id", message.User.RoomId),
									slog.String("username", message.User.Username),
									slog.Int("job", jobs),
								))
						case CONN_APPROVED:
							client.Status = CONN_APPROVED
							client.Send <- message
							jobs -= 1
							h.Logger.LogAttrs(ctx, slog.LevelDebug, "hub_broadcast",
								slog.Group("data",
									slog.String("message", "client join approved"),
									slog.String("room_id", message.User.RoomId),
									slog.String("username", message.User.Username),
									slog.Int("job", jobs),
								))

						}
					} else if client.IsRoomOwner {
						h.Logger.LogAttrs(ctx, slog.LevelDebug, "hub_broadcast",
							slog.Group("data",
								slog.String("message", "sending client confirmation to room owner"),
								slog.String("room_id", message.User.RoomId),
								slog.String("username", client.Username),
								slog.Int("job", jobs),
							))
						client.Send <- message
						jobs -= 1
					}
					if jobs == 0 {
						h.Logger.LogAttrs(ctx, slog.LevelDebug, "hub_broadcast",
							slog.Group("data",
								slog.String("message", "join proecss complete"),
								slog.String("room_id", message.User.RoomId),
								slog.String("username", client.Username),
								slog.Int("job", jobs),
							))
						break
					}
				}
				continue
			}
			for client := range clientInRoom {
				h.Logger.LogAttrs(ctx, slog.LevelDebug, "hub_broadcast",
					slog.Group("data",
						slog.String("message", "broadcast data to all client"),
						slog.String("room_id", message.User.RoomId),
						slog.String("username", client.Username),
					))
				if client.Status == CONN_PENDING {
					continue
				}
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(clientInRoom, client)
				}
			}
		case query := <-h.CheckRoomId:
			_, exist := h.Rooms[query.RoomId]
			h.Logger.LogAttrs(ctx, slog.LevelDebug, "hub_query",
				slog.Group("data",
					slog.String("message", "check room id"),
					slog.String("room_id", query.RoomId),
					slog.Bool("exist", exist),
				))
			query.Reply <- exist
		}
	}
}
