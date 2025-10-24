package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
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
	*slog.Logger
}

func NewApiHandler(hub *internalWs.Hub, upgrader websocket.Upgrader, log *slog.Logger) *ApiHandler {
	return &ApiHandler{
		Hub:      hub,
		Upgrader: upgrader,
		Logger:   log,
	}
}

func (ah *ApiHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	ctx := context.TODO()
	encoder := json.NewEncoder(w)

	roomId := r.PathValue("code")
	ah.Logger.LogAttrs(ctx, slog.LevelDebug, "request_join_room", slog.Group("data", slog.String("room_id", roomId)))
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
		ah.Logger.LogAttrs(ctx, slog.LevelDebug, "request_join_room", slog.Group("data", slog.Bool("room_exist", roomExist)))
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
		ah.Logger.LogAttrs(
			ctx,
			slog.LevelWarn,
			"request_join_room",
			slog.Group("error",
				slog.String("message", "fail to upgrade connection"),
				slog.Any("detail", err),
			),
		)
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
		Send:        make(chan *internalWs.WsPayload, 256),
		IsRoomOwner: false,
		Username:    internalWs.GenerateUsername(),
	}

	ah.Hub.Register <- &client

	go client.WritePump()
	go client.ReadPump()
}

func (ah *ApiHandler) ServeWs(w http.ResponseWriter, r *http.Request) {
	ctx := context.TODO()
	encoder := json.NewEncoder(w)
	conn, err := ah.Upgrader.Upgrade(w, r, nil)

	if err != nil {
		ah.Logger.LogAttrs(
			ctx,
			slog.LevelWarn,
			"request_create_room",
			slog.Group("error",
				slog.String("message", "fail to upgrade connection"),
				slog.Any("detail", err),
			),
		)
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
		ah.Logger.LogAttrs(
			ctx,
			slog.LevelWarn,
			"request_create_room",
			slog.Group("error",
				slog.String("message", "fail to generate random crypto number for room id"),
				slog.Any("detail", err),
			),
		)
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
		Send:        make(chan *internalWs.WsPayload, 256),
		RoomId:      RandomCrypto.String(),
		Status:      internalWs.CONN_APPROVED,
		Username:    internalWs.GenerateUsername(),
		IsRoomOwner: true,
	}
	client.Hub.Register <- client

	go client.WritePump()
	go client.ReadPump()
}

func main() {
	err := godotenv.Load()
	if err != nil {
		err = fmt.Errorf("fail to load env %w", err)
		panic(err)
	}
	var logLevel slog.Level = slog.LevelInfo
	if os.Getenv("ENVIRONMENT") != "production" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	mux := http.NewServeMux()

	allowedOrigins := map[string]bool{
		"http://localhost:5173": true,
	}
	hub := internalWs.NewHub(logger)
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			_, exist := allowedOrigins[r.Header.Get("origin")]
			return exist
		},
	}
	handler := NewApiHandler(hub, upgrader, logger)

	go hub.Run()

	mux.HandleFunc("/v1/rooms", handler.ServeWs)
	mux.HandleFunc("GET /v1/rooms/{code}", handler.JoinRoom)

	host := os.Getenv("HOST")
	server := &http.Server{
		Handler: &LoggerMiddleware{
			Handler: mux,
			Logger:  logger,
		},
		Addr: host,
	}
	logger.LogAttrs(context.TODO(), slog.LevelInfo, "start server at "+host)
	if err := server.ListenAndServe(); err != nil {
		logger.LogAttrs(context.TODO(), slog.LevelError, "fail to start server at"+host, slog.Any("error", err))
	}
}

type LoggerMiddleware struct {
	http.Handler
	*slog.Logger
}

func (lm *LoggerMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.TODO()
	lm.LogAttrs(ctx, slog.LevelInfo, "request", slog.Group("data",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("user_agent", r.UserAgent()),
		slog.String("ip", r.RemoteAddr),
	))
	lm.Handler.ServeHTTP(w, r)
}
