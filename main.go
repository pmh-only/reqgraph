package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	_ "embed"

	"github.com/gorilla/websocket"
)

//go:embed index.html
var indexHTML []byte

// Upgrader for WebSocket connections.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// For production, implement proper origin checks.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// RequestMessage represents a message sent from the frontend.
type RequestMessage struct {
	Type    string            `json:"type"`    // Must be "httpRequest"
	Method  string            `json:"method"`  // HTTP method: GET, POST, etc.
	URL     string            `json:"url"`     // Target URL
	Headers map[string]string `json:"headers"` // Optional headers
	Body    string            `json:"body"`    // Optional request body
}

// ResponseMessage represents the message sent back to the frontend.
type ResponseMessage struct {
	Type         string  `json:"type"`                   // "httpResponse"
	ResponseCode int     `json:"responseCode,omitempty"` // HTTP status code
	Latency      float64 `json:"latency,omitempty"`      // Latency in milliseconds
	Error        string  `json:"error,omitempty"`        // Error message if any
}

// wsHandler upgrades HTTP to WebSocket and handles incoming messages.
func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}
		var reqMsg RequestMessage
		if err := json.Unmarshal(msg, &reqMsg); err != nil {
			log.Println("JSON unmarshal error:", err)
			continue
		}
		if reqMsg.Type == "httpRequest" {
			processHTTPRequest(conn, reqMsg)
		}
	}
}

// processHTTPRequest performs the HTTP request and sends back the response.
func processHTTPRequest(conn *websocket.Conn, reqMsg RequestMessage) {
	start := time.Now()
	client := &http.Client{}

	var bodyReader io.Reader
	if reqMsg.Body != "" {
		bodyReader = strings.NewReader(reqMsg.Body)
	}

	request, err := http.NewRequest(reqMsg.Method, reqMsg.URL, bodyReader)
	if err != nil {
		sendError(conn, err.Error())
		return
	}

	for key, value := range reqMsg.Headers {
		request.Header.Set(key, value)
	}

	resp, err := client.Do(request)
	latency := time.Since(start).Seconds() * 1000 // Convert to milliseconds
	if err != nil {
		sendError(conn, err.Error())
		return
	}
	defer resp.Body.Close()

	response := ResponseMessage{
		Type:         "httpResponse",
		ResponseCode: resp.StatusCode,
		Latency:      latency,
	}
	sendResponse(conn, response)
}

func sendResponse(conn *websocket.Conn, resp ResponseMessage) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Println("JSON marshal error:", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Println("WebSocket write error:", err)
	}
}

func sendError(conn *websocket.Conn, errorMsg string) {
	resp := ResponseMessage{
		Type:  "httpResponse",
		Error: errorMsg,
	}
	sendResponse(conn, resp)
}

// indexHandler serves the embedded index.html.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(indexHTML)
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/", indexHandler)
	log.Println("Server started at http://localhost:8080/")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
