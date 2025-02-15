// inspectr_proxy.go

package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
)

// --- Embedded Static Files ---
//
//go:embed app/*
var embeddedAppFS embed.FS

// InspectrData represents the data schema for request/response capture.
type InspectrData struct {
	Method   string          `json:"method"`
	URL      string          `json:"url"`
	Server   string          `json:"server"`
	Path     string          `json:"path"`
	ClientIP string          `json:"clientIp"`
	Latency  int64           `json:"latency"` // in milliseconds
	Request  RequestDetails  `json:"request"`
	Response ResponseDetails `json:"response"`
}

// RequestDetails holds details of the incoming HTTP request.
type RequestDetails struct {
	Payload     string              `json:"payload"`
	Headers     map[string][]string `json:"headers"`
	QueryParams map[string][]string `json:"queryParams"`
	Timestamp   string              `json:"timestamp"`
}

// ResponseDetails holds details of the response.
type ResponseDetails struct {
	Payload       string              `json:"payload"`
	Headers       map[string][]string `json:"headers"`
	StatusCode    int                 `json:"statusCode"`
	StatusMessage string              `json:"statusMessage"`
	Timestamp     string              `json:"timestamp"`
}

// CloudEvent wraps the InspectrData in a CloudEvents envelope.
type CloudEvent struct {
	SpecVersion     string       `json:"specversion"`
	Type            string       `json:"type"`
	Source          string       `json:"source"`
	ID              string       `json:"id"`
	Time            string       `json:"time"`
	DataContentType string       `json:"datacontenttype"`
	Data            InspectrData `json:"data"`
}

// wrapInCloudEvent creates a CloudEvent envelope from InspectrData.
func wrapInCloudEvent(data InspectrData) CloudEvent {
	return CloudEvent{
		SpecVersion:     "1.0",
		Type:            "com.inspectr.http",
		Source:          "/inspectr-proxy",
		ID:              uuid.New().String(),
		Time:            time.Now().Format(time.RFC3339Nano),
		DataContentType: "application/json",
		Data:            data,
	}
}

// broadcast sends the CloudEvent via an HTTP POST to the broadcast URL.
func broadcast(broadcastURL string, data InspectrData) {
	cloudEvent := wrapInCloudEvent(data)
	postData, err := json.Marshal(cloudEvent)
	if err != nil {
		log.Println("Error marshaling cloud event:", err)
		return
	}

	req, err := http.NewRequest("POST", broadcastURL, bytes.NewBuffer(postData))
	if err != nil {
		log.Println("Error creating POST request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Inspectr broadcast error:", err)
		return
	}
	defer resp.Body.Close()
	// Drain response
	io.Copy(ioutil.Discard, resp.Body)
}

// getBgColor returns an ANSI background color code based on the status code.
func getBgColor(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "\x1b[42m" // Green
	case status >= 300 && status < 400:
		return "\x1b[44m" // Blue
	case status >= 400 && status < 500:
		return "\x1b[43m" // Yellow
	case status >= 500:
		return "\x1b[41m" // Red
	default:
		return ""
	}
}

// printLog outputs a summary of the InspectrData to the console with color coding.
func printLog(data InspectrData) {
	bgColor := getBgColor(data.Response.StatusCode)
	reset := "\x1b[0m"
	coloredStatus := fmt.Sprintf("%s%d%s", bgColor, data.Response.StatusCode, reset)
	fmt.Printf("%s - %s %s (%dms) - %s\n",
		coloredStatus,
		data.Method,
		data.Path,
		data.Latency,
		data.Request.Timestamp)
}

// --- SSE Implementation ---

var (
	sseClientsMu sync.Mutex
	sseClients   = make(map[string]chan string)
)

// sseHandler handles GET requests to /sse to establish an SSE connection.
func sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Write an initial comment to keep connection alive.
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	// Generate a unique client ID.
	clientID := uuid.New().String()
	msgChan := make(chan string)
	sseClientsMu.Lock()
	sseClients[clientID] = msgChan
	sseClientsMu.Unlock()
	//log.Printf("🟢 SSE client connected: %s, total clients: %d", clientID, len(sseClients))

	// Listen for messages and write them to the ResponseWriter.
	notify := r.Context().Done()
	for {
		select {
		case msg := <-msgChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-notify:
			sseClientsMu.Lock()
			delete(sseClients, clientID)
			sseClientsMu.Unlock()
			//log.Printf("🔴 SSE client disconnected: %s", clientID)
			return
		}
	}
}

// ssePostHandler handles POST requests to /sse to broadcast a message to all SSE clients.
func ssePostHandler(w http.ResponseWriter, r *http.Request) {
	var message interface{}
	if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	msgBytes, err := json.Marshal(message)
	if err != nil {
		http.Error(w, "Error processing message", http.StatusInternalServerError)
		return
	}
	broadcastSSERaw(string(msgBytes))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Broadcast sent"))
}

// broadcastSSERaw sends a raw JSON string to all connected SSE clients.
func broadcastSSERaw(msg string) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()
	for id, ch := range sseClients {
		select {
		case ch <- msg:
		default:
			log.Printf("SSE client %s channel full, skipping message", id)
		}
	}
}

// broadcastSSE wraps the InspectrData in a CloudEvent and broadcasts it to SSE clients.
func broadcastSSE(data InspectrData) {
	cloudEvent := wrapInCloudEvent(data)
	postData, err := json.Marshal(cloudEvent)
	if err != nil {
		log.Println("Error marshaling cloud event for SSE:", err)
		return
	}
	broadcastSSERaw(string(postData))
}

// --- Proxy Handler ---

// proxyHandler processes incoming requests. If a backend is configured,
// it forwards the request and captures the response; otherwise it returns 200 OK.
func proxyHandler(backendAddr, broadcastURL string, enablePrint, enableBroadcast, appModeEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Read and capture the request body.
		reqBodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		// Restore the body for further processing.
		r.Body = ioutil.NopCloser(bytes.NewBuffer(reqBodyBytes))

		// Variables for response details.
		var respPayload string
		var respHeaders map[string][]string
		var respStatusCode int
		var respStatusMessage string
		var respTimestamp string

		if backendAddr != "" {
			// Parse the backend address.
			parsedBackend, err := url.Parse(backendAddr)
			if err != nil {
				http.Error(w, "Invalid backend address", http.StatusInternalServerError)
				return
			}
			// Resolve the incoming request relative to the backend URL.
			backendURL := parsedBackend.ResolveReference(r.URL)

			// Create a new request for the backend.
			newReq, err := http.NewRequest(r.Method, backendURL.String(), bytes.NewReader(reqBodyBytes))
			if err != nil {
				http.Error(w, "Failed to create backend request", http.StatusInternalServerError)
				return
			}
			newReq.Header = r.Header.Clone()

			// Forward the request to the backend.
			resp, err := http.DefaultClient.Do(newReq)
			if err != nil {
				http.Error(w, "Failed to forward request", http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			// Copy backend response headers.
			for key, values := range resp.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.WriteHeader(resp.StatusCode)
			respStatusCode = resp.StatusCode
			respStatusMessage = http.StatusText(resp.StatusCode)

			// Capture the response body while sending it to the client.
			var buf bytes.Buffer
			tee := io.TeeReader(resp.Body, &buf)
			io.Copy(w, tee)
			respPayload = buf.String()
			respHeaders = resp.Header
			respTimestamp = time.Now().Format(time.RFC3339Nano)
		} else {
			// No backend: respond with 200 OK.
			respPayload = "OK"
			respStatusCode = http.StatusOK
			respStatusMessage = "OK"
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(respPayload))
			respHeaders = w.Header()
			respTimestamp = time.Now().Format(time.RFC3339Nano)
		}

		latency := time.Since(startTime).Milliseconds()

		// Extract client IP.
		clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			clientIP = r.RemoteAddr
		}

		// Build the InspectrData structure.
		data := InspectrData{
			Method: r.Method,
			URL:    r.URL.String(),
			Server: func() string {
				if backendAddr != "" {
					return backendAddr
				}
				return r.Host
			}(),
			Path:     r.URL.Path,
			ClientIP: clientIP,
			Latency:  latency,
			Request: RequestDetails{
				Payload:     string(reqBodyBytes),
				Headers:     r.Header,
				QueryParams: r.URL.Query(),
				Timestamp:   startTime.Format(time.RFC3339Nano),
			},
			Response: ResponseDetails{
				Payload:       respPayload,
				Headers:       respHeaders,
				StatusCode:    respStatusCode,
				StatusMessage: respStatusMessage,
				Timestamp:     respTimestamp,
			},
		}

		// Print log to terminal if enabled.
		if enablePrint {
			printLog(data)
		}

		// Broadcast via HTTP POST if enabled.
		if enableBroadcast && broadcastURL != "" {
			go broadcast(broadcastURL, data)
		}

		// Broadcast via internal SSE if app mode is enabled.
		if appModeEnabled {
			go broadcastSSE(data)
		}
	}
}

// --- Main Function ---

func main() {
	// Define command-line flags.
	listenAddr := flag.String("listen", ":8080", "Address to listen for incoming HTTP requests (proxy)")
	backendAddr := flag.String("backend", "", "Backend service address (host:port). If empty, returns 200 OK for any request.")
	broadcastURL := flag.String("broadcast", "", "Broadcast URL for sending Inspectr events (HTTP POST)")
	printLogs := flag.Bool("print", false, "Print logs to terminal")
	// App mode flags.
	appMode := flag.Bool("app", false, "Start Inspectr App (serve embedded static assets and SSE endpoints)")
	appPort := flag.String("appPort", "4004", "Port to serve the Inspectr App (default 4004)")
	flag.Parse()

	enableBroadcast := *broadcastURL != ""
	enablePrint := *printLogs

	// If app mode is enabled, start a separate server for the Inspectr App.
	if *appMode {
		// Get a sub-FS for the app folder so that the files appear at the FS root.
		appStatic, err := fs.Sub(embeddedAppFS, "app")
		if err != nil {
			log.Fatal("Failed to create sub filesystem: ", err)
		}

		appMux := http.NewServeMux()
		// SSE endpoint.
		appMux.HandleFunc("/api/sse", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				sseHandler(w, r)
			} else if r.Method == "POST" {
				ssePostHandler(w, r)
			} else {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}
		})
		// Serve embedded static assets from the sub filesystem at root.
		appMux.Handle("/", http.FileServer(http.FS(appStatic)))
		go func() {
			log.Printf("Inspectr App server listening on :%s", *appPort)
			if err := http.ListenAndServe(":"+*appPort, appMux); err != nil {
				log.Fatal("Inspectr App server error:", err)
			}
		}()
	}

	// Register the proxy handler on the main mux.
	http.HandleFunc("/", proxyHandler(*backendAddr, *broadcastURL, enablePrint, enableBroadcast, *appMode))
	log.Printf("Inspectr Proxy server listening on %s", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		log.Fatal("Inspectr Proxy server error:", err)
	}
}
