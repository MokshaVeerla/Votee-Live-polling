package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/gorilla/websocket"
)

var redisClient *redis.Client
var clients = make(map[string]map[*websocket.Conn]bool)
var clientsMutex sync.Mutex
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
type Poll struct {
	ID       string         `json:"id"`
	Question string         `json:"question"`
	Options  []string       `json:"options"`
	Votes    map[string]int `json:"votes"`
}
type VoteRequest struct {
	Option string `json:"option"`
}
func enableCORS(w http.ResponseWriter) {
    w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}
func homeHandler(w http.ResponseWriter, r *http.Request) {
    enableCORS(w)

    fmt.Fprintln(w, "Live Polling Backend is running!")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]string{
		"status": "ok",
	}

	json.NewEncoder(w).Encode(response)
}

func getPollHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if strings.HasSuffix(r.URL.Path, "/vote") {
		voteHandler(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/polls/")

	if path == "" {
		http.Error(w, "Poll ID is required", http.StatusBadRequest)
		return
	}

	pollData, err := redisClient.Get(
		context.Background(),
		"poll:"+path,
	).Result()

	if err == redis.Nil {
		http.Error(w, "Poll not found", http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, "Could not get poll", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, pollData)
}
func voteHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var vote VoteRequest

	err := json.NewDecoder(r.Body).Decode(&vote)
	if err != nil {
		http.Error(w, "Invalid vote", http.StatusBadRequest)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/polls/")
	path = strings.TrimSuffix(path, "/vote")

	if path == "" {
		http.Error(w, "Poll ID is required", http.StatusBadRequest)
		return
	}

	pollData, err := redisClient.Get(
		context.Background(),
		"poll:"+path,
	).Result()

	if err == redis.Nil {
		http.Error(w, "Poll not found", http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, "Could not get poll", http.StatusInternalServerError)
		return
	}

	var poll Poll

	err = json.Unmarshal([]byte(pollData), &poll)
	if err != nil {
		http.Error(w, "Could not read poll", http.StatusInternalServerError)
		return
	}

	if _, exists := poll.Votes[vote.Option]; !exists {
		http.Error(w, "Invalid option", http.StatusBadRequest)
		return
	}

	poll.Votes[vote.Option]++

	updatedPoll, err := json.Marshal(poll)
	if err != nil {
		http.Error(w, "Could not update poll", http.StatusInternalServerError)
		return
	}

	err = redisClient.Set(
		context.Background(),
		"poll:"+path,
		updatedPoll,
		0,
	).Err()

	if err != nil {
		http.Error(w, "Could not save vote", http.StatusInternalServerError)
		return
	}

	broadcastMessage(path, updatedPoll)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(poll)
}

func createPollHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var poll Poll

	err := json.NewDecoder(r.Body).Decode(&poll)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	poll.ID = uuid.New().String()

	poll.Votes = make(map[string]int)

	for _, option := range poll.Options {
		poll.Votes[option] = 0
	}

	pollData, err := json.Marshal(poll)
	if err != nil {
		http.Error(w, "Could not create poll", http.StatusInternalServerError)
		return
	}

	err = redisClient.Set(
		context.Background(),
		"poll:"+poll.ID,
		pollData,
		0,
	).Err()

	if err != nil {
		http.Error(w, "Could not save poll", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(poll)
}
func websocketHandler(w http.ResponseWriter, r *http.Request) {
	pollID := strings.TrimPrefix(r.URL.Path, "/ws/")
	
	if pollID == "" {
		http.Error(w, "Poll ID is required", http.StatusBadRequest)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WebSocket upgrade failed:", err)
		return
	}

	defer conn.Close()

	fmt.Println("WebSocket client connected")

	clientsMutex.Lock()
	if clients[pollID] == nil {
		clients[pollID] = make(map[*websocket.Conn]bool)
	}
	clients[pollID][conn] = true
	clientsMutex.Unlock()

	err = conn.WriteMessage(
		websocket.TextMessage,
		[]byte("Connected to Live Polling Server!"),
	)

	if err != nil {
		fmt.Println("Could not send message:", err)
		return
	}

	for {
		_, _, err := conn.ReadMessage()

		if err != nil {
			fmt.Println("WebSocket client disconnected")

			clientsMutex.Lock()
			delete(clients[pollID], conn)
			clientsMutex.Unlock()

			break
		}
	}
}
func broadcastMessage(pollID string, message []byte) {
    clientsMutex.Lock()
    defer clientsMutex.Unlock()

    for client := range clients[pollID] {
        err := client.WriteMessage(websocket.TextMessage, message)

        if err != nil {
            fmt.Println("Could not send message:", err)
        }
    }
}

func main() {
	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		fmt.Println("Redis connection failed:", err)
		return
	}

	fmt.Println("Connected to Redis")
	fmt.Println("Server started at http://localhost:8080")

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/polls", createPollHandler)
	http.HandleFunc("/polls/", getPollHandler)
	http.HandleFunc("/ws/", websocketHandler)

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Server error:", err)
	}
}