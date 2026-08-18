package main

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

// DeviceInfo holds the latest cached state of an individual meter
type DeviceInfo struct {
	LatestCurrent   float64 `json:"latest_current"`
	Unit            string  `json:"unit"`
	LastRawResponse string  `json:"last_raw_response"`
	LastUpdate      int64   `json:"last_update"`
	DeviceID        string  `json:"device_id"` // Holds the instrument custom signature name
}

var (
	registry        = make(map[string]*DeviceInfo)
	registryLock    sync.RWMutex
	activePorts     = make(map[string]serial.Port)
	activePortsLock sync.Mutex

	runningWorkers     = make(map[string]bool)
	runningWorkersLock sync.Mutex
)

const sharedSecretKey = "YourSuperSecretPassword123!"

func discoverPorts() []string {
	matches, _ := filepath.Glob("/dev/ttyUSB*")
	matches2, _ := filepath.Glob("/dev/ttyACM*")
	return append(matches, matches2...)
}

func deviceWorker(portPath string) {
	fmt.Printf("[Worker] Tracking stream loop launched for: %s\n", portPath)

	mode := &serial.Mode{
		BaudRate: 9600,
	}

	port, err := serial.Open(portPath, mode)
	if err != nil {
		fmt.Printf("[Error] Cannot link port %s: %v\n", portPath, err)
		runningWorkersLock.Lock()
		delete(runningWorkers, portPath)
		runningWorkersLock.Unlock()
		return
	}

	defer func() {
		port.Close()
		activePortsLock.Lock()
		delete(activePorts, portPath)
		activePortsLock.Unlock()

		registryLock.Lock()
		delete(registry, portPath)
		registryLock.Unlock()

		runningWorkersLock.Lock()
		delete(runningWorkers, portPath)
		runningWorkersLock.Unlock()
		fmt.Printf("[Worker] Cleaned up and exited for port: %s\n", portPath)
	}()

	activePortsLock.Lock()
	activePorts[portPath] = port
	activePortsLock.Unlock()

	// 1. Initialize empty registry structure with a default placeholder ID name
	cleanName := strings.ToUpper(filepath.Base(portPath))
	registryLock.Lock()
	registry[portPath] = &DeviceInfo{Unit: "nA", DeviceID: cleanName, LastRawResponse: "Initializing instrument handshake..."}
	registryLock.Unlock()

	reader := bufio.NewReader(port)

	// 2. QUERY INSTRUMENT IDENTIFICATION BEFORE STARTUP
	time.Sleep(1 * time.Second)
	_, _ = port.Write([]byte("ID?\r\n"))
	
	// Read the unique identifier response line with a 2-second timeout window
	idLine, idErr := reader.ReadString('\n')
	customID := cleanName
	if idErr == nil {
		idLine = strings.TrimSpace(idLine)
		if idLine != "" {
			customID = fmt.Sprintf("%s - %s", cleanName, idLine)
			fmt.Printf("[Worker] Identified device on %s as: %s\n", portPath, customID)
		}
	}

	// Update registry with the newly acquired custom device ID header tag
	registryLock.Lock()
	registry[portPath].DeviceID = customID
	registryLock.Unlock()

	// 3. LAUNCH NORMAL SYSTEM MEASUREMENT READINGS STREAM
	time.Sleep(1 * time.Second)
	if _, err := port.Write([]byte("START\r\n")); err != nil {
		fmt.Printf("[Error] Failed to send START to %s: %v\n", portPath, err)
		return
	}

	registryLock.Lock()
	registry[portPath].LastRawResponse = "Connected, awaiting data..."
	registryLock.Unlock()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("[Worker] Device disconnected on port: %s\n", portPath)
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Simplified Parsing: Match generic "I =" data structures directly
		if strings.HasPrefix(line, "I") && strings.Contains(line, "=") {
			detectedUnit := "nA"
			if strings.HasSuffix(line, "uA") {
				detectedUnit = "uA"
			}

			// Slice away the generic "I =" baseline token definition prefix cleanly
			idx := strings.Index(line, "=")
			cleanStr := line[idx+1:]
			cleanStr = strings.ReplaceAll(cleanStr, "nA", "")
			cleanStr = strings.ReplaceAll(cleanStr, "uA", "")
			cleanStr = strings.TrimSpace(cleanStr)

			var parsedVal float64
			if _, sscanfErr := fmt.Sscanf(cleanStr, "%f", &parsedVal); sscanfErr == nil {
				registryLock.Lock()
				if dev, exists := registry[portPath]; exists {
					dev.LatestCurrent = parsedVal
					dev.Unit = detectedUnit
					dev.LastUpdate = time.Now().Unix()
					dev.LastRawResponse = ""
				}
				registryLock.Unlock()
			}

		// Simplified Parsing: Ignore generic "C =" configuration data structures
		} else if strings.HasPrefix(line, "C") && strings.Contains(line, "=") {
			continue

		// Asynchronous diagnostic update responses lines stream buffer push
		} else {
			registryLock.Lock()
			if dev, exists := registry[portPath]; exists {
				dev.LastRawResponse = line
				dev.LastUpdate = time.Now().Unix()
			}
			registryLock.Unlock()
		}
	}
}

func monitorTopologyLoop() {
	for {
		availablePorts := discoverPorts()
		runningWorkersLock.Lock()
		for _, port := range availablePorts {
			if !runningWorkers[port] {
				runningWorkers[port] = true
				go deviceWorker(port)
			}
		}
		runningWorkersLock.Unlock()
		time.Sleep(5 * time.Second)
	}
}

func setupHTTPResponse(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

func handleDevices(w http.ResponseWriter, r *http.Request) {
	if setupHTTPResponse(w, r) {
		return
	}
	_ = json.NewEncoder(w).Encode(discoverPorts())
}

func handleCurrent(w http.ResponseWriter, r *http.Request) {
	if setupHTTPResponse(w, r) {
		return
	}

	targetPort := r.URL.Query().Get("port")

	registryLock.RLock()
	info, exists := registry[targetPort]
	var responseData DeviceInfo
	if exists {
		responseData = *info
	} else {
		cleanName := strings.ToUpper(filepath.Base(targetPort))
		responseData = DeviceInfo{LatestCurrent: 0.0, Unit: "nA", DeviceID: cleanName, LastRawResponse: "Port uninitialized"}
	}
	registryLock.RUnlock()

	_ = json.NewEncoder(w).Encode(responseData)
}

func handleCommand(w http.ResponseWriter, r *http.Request) {
	if setupHTTPResponse(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	clientSignature := r.Header.Get("X-Gateway-Signature")
	if clientSignature == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	mac := hmac.New(sha256.New, []byte(sharedSecretKey))
	mac.Write(bodyBytes)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(clientSignature), []byte(expectedSignature)) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var payload struct {
		Port    string `json:"port"`
		Command string `json:"command"`
	}
	if err = json.Unmarshal(bodyBytes, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	activePortsLock.Lock()
	port, exists := activePorts[payload.Port]
	activePortsLock.Unlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	_, err = port.Write([]byte(payload.Command + "\r\n"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "Success"})
}

func main() {
	go monitorTopologyLoop()

	http.HandleFunc("/api/devices", handleDevices)
	http.HandleFunc("/api/current", handleCurrent)
	http.HandleFunc("/api/command", handleCommand)

	fmt.Println("Pi Gateway Server listening natively on Port 2000...")
	_ = http.ListenAndServe(":2000", nil)
}

