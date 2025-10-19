package websocket

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const (
	CREATE_ROOM   = "create-room"
	JOIN_ROOM     = "join-room"
	CONN_APPROVED = "approved"
	CONN_PENDING  = "pending"
	CONN_REJECTED = "rejected"
)

// middleman between websocket connection and the hub
// represent one single client connection
type Client struct {
	Hub  *Hub
	Conn *websocket.Conn

	// buffered channel of outbound message
	Send chan *WsPayload[WsData]

	RoomId string

	// status connection e.g approved, pending, rejected
	// join room operation will check if new connection in a room is approved or not
	Status string

	IsRoomOwner bool

	Username string
}

type WsData struct {
	RoomId   string `json:"room_id"`
	Username string `json:"username"`
}

type WsPayload[T any] struct {
	Type string `json:"type"`
	// give user info wether the connection is success, pending, fail or rejected
	Status  string `json:"status"`
	Data    T      `json:"data,omitempty"`
	IsOwner bool   `json:"is_owner"`
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	for {
		data := &WsPayload[WsData]{}
		err := c.Conn.ReadJSON(data)
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
			}
			break
		}

		// create a message and send it to hub broadcast channel
		c.Hub.Broadcast <- data
	}
}

func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				// the hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteJSON(message)
		}
	}
}

func GenerateUsername() string {
	fruits := []string{
		"apple", "orange", "eggplant", "carrot", "cabbage",
		"pineapple", "grape", "strawberry", "blueberry", "kiwi",
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	min := 10000
	max := 99999
	randomNumber := r.Intn(max-min+1) + min

	numberStr := strconv.Itoa(randomNumber)

	fruitIndex := len(numberStr) - 1
	selectedFruit := fruits[fruitIndex]

	return fmt.Sprintf("%s-%s", selectedFruit, numberStr)
}
