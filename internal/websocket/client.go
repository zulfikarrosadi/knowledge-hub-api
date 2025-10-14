package websocket

import (
	"log"

	"github.com/gorilla/websocket"
)

// middleman between websocket connection and the hub
// represent one single client connection
type Client struct {
	Hub  *Hub
	Conn *websocket.Conn

	// buffered channel of outbound message
	Send chan []byte

	RoomId string
}

// Message is the object that will be sent to broadcast channel
type Message struct {
	Data   []byte
	RoomId string
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// create a message and send it to hub broadcast channel
		msg := &Message{
			Data:   data,
			RoomId: c.RoomId,
		}
		c.Hub.Broadcast <- msg
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
			c.Conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}
