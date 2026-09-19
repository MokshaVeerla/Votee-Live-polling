package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Poll struct {
	ID       string         `json:"id" bson:"id"`
	Question string         `json:"question" bson:"question"`
	Options  []string       `json:"options" bson:"options"`
	Votes    map[string]int `json:"votes" bson:"votes"`
}

type VoteRequest struct {
	Option string `json:"option"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var redisClient *redis.Client
var mongoClient *mongo.Client
var pollCollection *mongo.Collection

var authUsername string
var authPassword string
var authToken string
var corsOrigin string

var (
	clients   = make(map[string]map[*websocket.Conn]bool)
	clientsMu sync.Mutex
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// getEnv returns the environment variable value.
// If it is not set, it uses the provided fallback value.
func getEnv(key string, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

func enableCORS(c *gin.Context) {
	c.Header(
		"Access-Control-Allow-Origin",
		corsOrigin,
	)

	c.Header(
		"Access-Control-Allow-Headers",
		"Content-Type, Authorization",
	)

	c.Header(
		"Access-Control-Allow-Methods",
		"GET, POST, OPTIONS",
	)

	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}

	c.Next()
}

func homeHandler(c *gin.Context) {
	c.String(
		http.StatusOK,
		"Live Polling Tool Backend is running",
	)
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func loginHandler(c *gin.Context) {
	var login LoginRequest

	if err := c.ShouldBindJSON(&login); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request",
		})
		return
	}

	if login.Username != authUsername ||
		login.Password != authPassword {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid username or password",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Login successful",
		"username": login.Username,
		"token":    authToken,
	})
}

func authMiddleware(c *gin.Context) {
	token := c.GetHeader("Authorization")

	if token != "Bearer "+authToken {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})

		c.Abort()
		return
	}

	c.Next()
}

func createPollHandler(c *gin.Context) {
	var input struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request",
		})
		return
	}

	input.Question = strings.TrimSpace(input.Question)

	if input.Question == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Question cannot be empty",
		})
		return
	}

	if len(input.Options) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "At least two options are required",
		})
		return
	}

	cleanOptions := make([]string, 0, len(input.Options))

	for _, option := range input.Options {
		option = strings.TrimSpace(option)

		if option == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Options cannot be empty",
			})
			return
		}

		cleanOptions = append(cleanOptions, option)
	}

	votes := make(map[string]int)

	for _, option := range cleanOptions {
		votes[option] = 0
	}

	poll := Poll{
		ID:       uuid.New().String(),
		Question: input.Question,
		Options:  cleanOptions,
		Votes:    votes,
	}

	_, err := pollCollection.InsertOne(
		context.Background(),
		poll,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Could not save poll to MongoDB",
		})
		return
	}

	// Create live vote counters in Redis.
	for _, option := range cleanOptions {
		err := redisClient.Set(
			context.Background(),
			"poll:"+poll.ID+":votes:"+option,
			0,
			0,
		).Err()

		if err != nil {
			fmt.Println("Redis counter error:", err)
		}
	}

	c.JSON(http.StatusCreated, poll)
}

func getPollHandler(c *gin.Context) {
	pollID := c.Param("id")

	var poll Poll

	err := pollCollection.FindOne(
		context.Background(),
		bson.M{"id": pollID},
	).Decode(&poll)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Poll not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Could not retrieve poll",
		})
		return
	}

	c.JSON(http.StatusOK, poll)
}

func voteHandler(c *gin.Context) {
	pollID := c.Param("id")

	var vote VoteRequest

	if err := c.ShouldBindJSON(&vote); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid vote request",
		})
		return
	}

	vote.Option = strings.TrimSpace(vote.Option)

	if vote.Option == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Option is required",
		})
		return
	}

	// Update the permanent vote count in MongoDB.
	updateResult, err := pollCollection.UpdateOne(
		context.Background(),
		bson.M{
			"id": pollID,
			"votes." + vote.Option: bson.M{
				"$exists": true,
			},
		},
		bson.M{
			"$inc": bson.M{
				"votes." + vote.Option: 1,
			},
		},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Could not record vote",
		})
		return
	}

	if updateResult.MatchedCount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid poll or option",
		})
		return
	}

	// Update the live counter in Redis.
	redisKey := "poll:" + pollID + ":votes:" + vote.Option

	_, err = redisClient.Incr(
		context.Background(),
		redisKey,
	).Result()

	if err != nil {
		fmt.Println("Redis counter error:", err)
	}

	// Get the latest poll from MongoDB.
	var updatedPoll Poll

	err = pollCollection.FindOne(
		context.Background(),
		bson.M{"id": pollID},
	).Decode(&updatedPoll)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Could not retrieve updated poll",
		})
		return
	}

	// Send the latest poll through Redis Pub/Sub.
	message, err := json.Marshal(updatedPoll)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Could not prepare live update",
		})
		return
	}

	err = redisClient.Publish(
		context.Background(),
		"poll:"+pollID,
		message,
	).Err()

	if err != nil {
		fmt.Println("Redis publish error:", err)
	}

	c.JSON(http.StatusOK, updatedPoll)
}

func websocketHandler(c *gin.Context) {
	pollID := c.Param("id")

	conn, err := upgrader.Upgrade(
		c.Writer,
		c.Request,
		nil,
	)

	if err != nil {
		return
	}

	clientsMu.Lock()

	if clients[pollID] == nil {
		clients[pollID] = make(map[*websocket.Conn]bool)
	}

	clients[pollID][conn] = true

	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()

		delete(clients[pollID], conn)

		if len(clients[pollID]) == 0 {
			delete(clients, pollID)
		}

		clientsMu.Unlock()

		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()

		if err != nil {
			break
		}
	}
}

func broadcastMessage(
	pollID string,
	message []byte,
) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for conn := range clients[pollID] {
		err := conn.WriteMessage(
			websocket.TextMessage,
			message,
		)

		if err != nil {
			conn.Close()
			delete(clients[pollID], conn)
		}
	}
}

func redisSubscriber() {
	for {
		pubsub := redisClient.PSubscribe(
			context.Background(),
			"poll:*",
		)

		_, err := pubsub.Receive(
			context.Background(),
		)

		if err != nil {
			fmt.Println(
				"Redis subscription error:",
				err,
			)

			pubsub.Close()

			time.Sleep(2 * time.Second)

			continue
		}

		ch := pubsub.Channel()

		for message := range ch {
			pollID := strings.TrimPrefix(
				message.Channel,
				"poll:",
			)

			broadcastMessage(
				pollID,
				[]byte(message.Payload),
			)
		}

		pubsub.Close()

		time.Sleep(1 * time.Second)
	}
}

func main() {
	// Application configuration.
	redisAddress := getEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	mongoURI := getEnv(
		"MONGO_URI",
		"mongodb://127.0.0.1:27017",
	)

	corsOrigin = getEnv(
		"CORS_ORIGIN",
		"http://localhost:5173",
	)

	authUsername = getEnv(
		"AUTH_USERNAME",
		"admin",
	)

	authPassword = getEnv(
		"AUTH_PASSWORD",
		"admin123",
	)

	authToken = getEnv(
		"AUTH_TOKEN",
		"live-poll-admin-token",
	)

	port := getEnv(
		"PORT",
		"8080",
	)

	// Connect to Redis.
	redisOptions := &redis.Options{
		Addr:      redisAddress,
		Password:  redisPassword,
	}
	if redisPassword != "" {
		redisOptions.TLSConfig = &tls.Config{
			ServerName: "heroic-joey-285292.upstash.io",
		}
	}
	redisClient = redis.NewClient(redisOptions)

	_, err := redisClient.Ping(
		context.Background(),
	).Result()

	if err != nil {
		fmt.Println("Redis connection failed:", err)
		return
	}

	fmt.Println("Connected to Redis")

	// Connect to MongoDB.
	mongoClient, err = mongo.Connect(
		options.Client().ApplyURI(
			mongoURI,
		),
	)

	if err != nil {
		fmt.Println("MongoDB connection failed:", err)
		return
	}

	err = mongoClient.Ping(
		context.Background(),
		nil,
	)

	if err != nil {
		fmt.Println("MongoDB ping failed:", err)
		return
	}

	fmt.Println("Connected to MongoDB")

	pollCollection = mongoClient.
		Database("livepoll").
		Collection("polls")

	// Start Redis listener.
	go redisSubscriber()

	// Use Gin's release mode.
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// Do not trust arbitrary proxy addresses.
	router.SetTrustedProxies(nil)

	router.Use(enableCORS)

	router.GET("/", homeHandler)

	router.GET("/health", healthHandler)

	router.POST("/login", loginHandler)

	router.POST(
		"/polls",
		authMiddleware,
		createPollHandler,
	)

	router.GET(
		"/polls/:id",
		getPollHandler,
	)

	router.POST(
		"/polls/:id/vote",
		voteHandler,
	)

	router.GET(
		"/ws/:id",
		websocketHandler,
	)

	fmt.Println(
		"Server started at http://localhost:" + port,
	)

	err = router.Run(":" + port)

	if err != nil {
		fmt.Println("Server error:", err)
	}
}
