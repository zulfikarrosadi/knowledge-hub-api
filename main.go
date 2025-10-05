package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		if r.Header.Get("origin") == "http://localhost:5173" {
			return true
		}
		return false
	},
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}

	roomId := strings.TrimPrefix(r.URL.Path, "/ws/")
	if roomId == "" {
		log.Println("room id is required: ", roomId)
		conn.Close()
	}

	client := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		roomId: roomId,
	}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func main() {
	hub := newHub()
	go hub.run()

	http.HandleFunc("/ws/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	log.Println("Server starting on :3000")
	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalf("fail to start at port 3000 %v", err)
	}
}

// middleman between websocket connection and the hub
// represent one single client connection
type Client struct {
	hub  *Hub
	conn *websocket.Conn

	// buffered channel of outbound message
	send chan []byte

	roomId string
}

// Message is the object that will be sent to broadcast channel
type Message struct {
	Data   []byte
	RoomId string
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// create a message and send it to hub broadcast channel
		msg := &Message{
			Data:   data,
			RoomId: c.roomId,
		}
		c.hub.broadcast <- msg
	}
}

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// the hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

// Hub maintains the set of active clients and broadcasts messages to the clients.
// it's the manager to let each connection to communicate to the same websocket
type Hub struct {
	rooms map[string]map[*Client]bool

	// to send the message to the same websocket connection
	broadcast chan *Message

	// to add new client / new websocket connection to the map
	register chan *Client

	// to remove websocket connection from the map
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan *Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		rooms:      make(map[string]map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			if _, exist := h.rooms[client.roomId]; !exist {
				h.rooms[client.roomId] = make(map[*Client]bool)
			}
			h.rooms[client.roomId][client] = true
			log.Printf("Client registered to room %s", client.roomId)
		case client := <-h.unregister:
			if _, exist := h.rooms[client.roomId]; exist {
				delete(h.rooms[client.roomId], client)
				close(client.send)
			}
			if len(h.rooms[client.roomId]) == 0 {
				delete(h.rooms, client.roomId)
			}
			log.Printf("Client unregistered from room %s", client.roomId)
		case message := <-h.broadcast:
			if clientInRoom, exist := h.rooms[message.RoomId]; exist {
				for client := range clientInRoom {
					select {
					case client.send <- message.Data:
					default:
						close(client.send)
						delete(clientInRoom, client)
					}
				}
			}
		}
	}
}
