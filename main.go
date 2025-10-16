package main

import (
	"crypto/rand"
	"encoding/json"
	"log"
	"math/big"
	"net/http"

	"github.com/gorilla/websocket"
	internalWs "github.com/zulfikarrosadi/knowledge-hub-api/internal/websocket"
)

type ApiError struct {
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type ApiRes[T any] struct {
	Status string   `json:"status"`
	Code   int      `json:"code"`
	Data   T        `json:"data,omitempty"`
	Error  ApiError `json:"error,omitempty"`
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
	encoder := json.NewEncoder(w)

	roomId := r.PathValue("code")
	if roomId == "" {
		res := ApiRes[any]{
			Status: "fail",
			Code:   http.StatusBadRequest,
			Error: ApiError{
				Message: "Fail to join room, please enter correct room code",
			},
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(res.Code)
		if err := encoder.Encode(res); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	queryRoom := internalWs.QueryRoomId{
		RoomId: roomId,
		Reply:  make(chan bool),
	}
	ah.Hub.CheckRoomId <- &queryRoom
	if roomExist := <-queryRoom.Reply; !roomExist {
		res := ApiRes[any]{
			Status: "fail",
			Code:   http.StatusNotFound,
			Error: ApiError{
				Message: "Room not exist yet",
			},
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(res.Code)
		if err := encoder.Encode(res); err != nil {
			w.WriteHeader(http.StatusBadGateway)
		}
		return
	}

	conn, err := ah.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		res := ApiRes[any]{
			Status: "fail",
			Code:   http.StatusInternalServerError,
			Error: ApiError{
				Message: "Fail to create connection, please try again later",
			},
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(res.Code)
		if err = encoder.Encode(res); err != nil {
			w.WriteHeader(http.StatusBadGateway)
		}
		return
	}

	client := internalWs.Client{
		Hub:         ah.Hub,
		Conn:        conn,
		RoomId:      roomId,
		Status:      internalWs.CONN_PENDING,
		Send:        make(chan *internalWs.WsPayload[internalWs.WsData], 256),
		IsRoomOwner: false,
	}

	ah.Hub.Register <- &client

	go client.WritePump()
	go client.ReadPump()
}

func (ah *ApiHandler) ServeWs(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)
	conn, err := ah.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		res := ApiRes[any]{
			Status: "fail",
			Code:   http.StatusInternalServerError,
			Error: ApiError{
				Message: "Fail to create connection, please try again later",
			},
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(res.Code)
		if err = encoder.Encode(res); err != nil {
			w.WriteHeader(http.StatusBadGateway)
		}
		return
	}

	RandomCrypto, err := rand.Int(rand.Reader, big.NewInt(128))
	if err != nil {
		res := ApiRes[any]{
			Status: "fail",
			Code:   http.StatusInternalServerError,
			Error: ApiError{
				Message: "Fail to create connection, please try again later",
			},
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(res.Code)
		if err = encoder.Encode(res); err != nil {
			w.WriteHeader(http.StatusBadGateway)
		}
		return
	}

	client := &internalWs.Client{
		Hub:         ah.Hub,
		Conn:        conn,
		Send:        make(chan *internalWs.WsPayload[internalWs.WsData], 256),
		RoomId:      RandomCrypto.String(),
		Status:      internalWs.CONN_APPROVED,
		IsRoomOwner: true,
	}
	client.Hub.Register <- client

	go client.WritePump()
	go client.ReadPump()
}

func main() {
	allowedOrigins := map[string]bool{
		"http://localhost:5173": true,
	}

	hub := internalWs.NewHub()
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			_, exist := allowedOrigins[r.Header.Get("origin")]
			return exist
		},
	}
	handler := NewApiHandler(hub, upgrader)

	go hub.Run()

	http.HandleFunc("/v1/rooms", handler.ServeWs)
	http.HandleFunc("GET v1/rooms/{code}", handler.JoinRoom)

	log.Println("Server starting on :3000")
	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalf("fail to start at port 3000 %v", err)
	}
}
