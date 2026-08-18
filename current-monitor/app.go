package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const sharedSecretKey = "1ConnectMeToDevices!"

type App struct {
	ctx context.Context
}

func NewApp() *App { return &App{} }
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// Keep the old legacy layout handle to avoid any compilation breaking dependencies
func (a *App) FetchCurrent() float64 { return 0.0 }

// Explicitly mirror the Pi Gateway backend's payload data shape
type DeviceState struct {
	LatestCurrent   float64 `json:"latest_current"`
	Unit            string  `json:"unit"`
	LastRawResponse string  `json:"last_raw_response"`
	LastUpdate      int64   `json:"last_update"` 
}

// Helper utility to make sure a port is attached to the IP string
func formatTargetURL(ip string, endpoint string) string {
	// If the user forgot to add port 2000 in the UI box, append it automatically
	if !strings.Contains(ip, ":") {
		ip = ip + ":2000"
	}
	return fmt.Sprintf("http://%s%s", ip, endpoint)
}

// FetchDeviceList contacts the Pi gateway to discover all connected paths
func (a *App) FetchDeviceList(ip string) ([]string, error) {
	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(formatTargetURL(ip, "/api/devices"))
	if err != nil {
		return []string{}, err
	}
	defer resp.Body.Close()

	var list []string
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return []string{}, err // Return parsing failures cleanly
	}
	return list, nil
}

// FetchMeterData reads the specific data map for a single device port path
func (a *App) FetchMeterData(ip string, portPath string) (DeviceState, error) {
	client := http.Client{Timeout: 1 * time.Second}
	url := fmt.Sprintf("%s?port=%s", formatTargetURL(ip, "/api/current"), portPath)
	
	resp, err := client.Get(url)
	if err != nil {
		return DeviceState{}, err
	}
	defer resp.Body.Close()

	var state DeviceState
	err = json.NewDecoder(resp.Body).Decode(&state)
	return state, err
}

// SendCustomCommand handles delivering instructions from the terminal box input
func (a *App) SendCustomCommand(ip string, portPath string, commandStr string) (string, error) {
	client := http.Client{Timeout: 2 * time.Second}
	payload := map[string]string{"port": portPath, "command": commandStr}
	jsonBytes, _ := json.Marshal(payload)

	// 1. Compute the HMAC token based on the raw JSON request content
	mac := hmac.New(sha256.New, []byte(sharedSecretKey))
	mac.Write(jsonBytes)
	signatureToken := hex.EncodeToString(mac.Sum(nil))

	// 2. Build the request manually to attach our security header safely
	req, err := http.NewRequest(http.MethodPost, formatTargetURL(ip, "/api/command"), bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gateway-Signature", signatureToken) // Inject token header

	// 3. Execute the network call
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res["status"], nil
}


