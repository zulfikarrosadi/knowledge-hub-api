package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	internalWs "github.com/zulfikarrosadi/knowledge-hub-api/internal/websocket"
)

type ApiRes struct {
	Status string `json:"status"`
}

type WsMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type ApiHandler struct {
	*internalWs.Hub
	websocket.Upgrader
}

func NewApiHandler(hub *internalWs.Hub, upgrader websocket.Upgrader) *ApiHandler {
	return &ApiHandler{
		Hub:      hub,
		Upgrader: upgrader,
	}
}

func (ah *ApiHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var res ApiRes

	encoder := json.NewEncoder(w)
	err := encoder.Encode(&res)

	if err != nil {
		w.WriteHeader(400)
		return
	}
}

func (ah *ApiHandler) ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := ah.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}

	roomId := strings.TrimPrefix(r.URL.Path, "/ws/")
	if roomId == "" {
		log.Println("room id is required: ", roomId)
		conn.Close()
	}

	client := &internalWs.Client{
		Hub:    ah.Hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		RoomId: roomId,
	}
	client.Hub.Register <- client

	go client.WritePump()
	go client.ReadPump()
}

func main() {
	hub := internalWs.NewHub()
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if r.Header.Get("origin") == "http://localhost:5173" {
				return true
			}
			return false
		},
	}
	handler := NewApiHandler(hub, upgrader)

	go hub.Run()

	http.HandleFunc("/ws/", handler.ServeWs)
	http.HandleFunc("GET v1/rooms/{code}", handler.JoinRoom)

	log.Println("Server starting on :3000")
	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalf("fail to start at port 3000 %v", err)
	}
}
