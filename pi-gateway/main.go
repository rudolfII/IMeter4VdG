package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fernet/fernet-go"
	"go.bug.st/serial"
)

type DeviceInfo struct {
	LatestCurrent   string `json:"latest_current"`
	LastAns         string `json:"last_ans"`
	TimeLastConnect int    `json:"time_last_connect"`
	LastUpdate      time.Time
}

var (
	registry     = make(map[string]*DeviceInfo)
	registryLock sync.RWMutex
	activePorts  = make(map[string]serial.Port)
	portsLock    sync.Mutex

	commandMailbox = make(map[string]string)
	mailboxLock    sync.Mutex
)

const psk = "9AiX1wUIPnZYmaVDkCFI8c4nikAne-cZgfGGd_BdOA4="

func discoverPorts() []string {
	matches, _ := filepath.Glob("/dev/ttyUSB*")
	matches2, _ := filepath.Glob("/dev/ttyACM*")
	return append(matches, matches2...)
}

func deviceWorker(portPath string) {
	fmt.Printf("[Worker] Tracking loop launched for: %s\n", portPath)
	var meterName string

	mode := &serial.Mode{
		BaudRate: 9600,
	}

	port, err := serial.Open(portPath, mode)
	if err != nil {
		fmt.Printf("[Error] Cannot link port %s: %v\n", portPath, err)
		return
	}

	portsLock.Lock()
	activePorts[portPath] = port
	portsLock.Unlock()

	defer func() {
		port.Close()
		portsLock.Lock()
		delete(activePorts, portPath)
		portsLock.Unlock()

		registryLock.Lock()
		if meterName != "" { delete(registry, meterName) }
		registryLock.Unlock()
		fmt.Printf("[Worker] Cleaned up and exited for port: %s\n", portPath)
	}()

	time.Sleep(2 * time.Second)
	
	reader := bufio.NewReader(port)

	_, _ = port.Write([]byte("id\r"))
	for i := 0; i < 5; i++ {
		line, err := reader.ReadString('\r')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "I_METER_") {
			meterName = strings.Replace(line, "I_METER_", "", 1)
			break
		}
	}

	if meterName == "" {
		fmt.Printf("[-] Failed to identify smart meter protocol on %s. Closing.\n", portPath)
		return
	}

	fmt.Printf("[+] Successfully Identified: %s on %s\n", meterName, portPath)

	registryLock.Lock()
	registry[meterName] = &DeviceInfo{
		LatestCurrent: "No data yet\n",
		LastAns:       "No answer yet\n",
		LastUpdate:    time.Now(),
	}
	registryLock.Unlock()

	for {
		registryLock.RLock()
		devExists := registry[meterName] != nil
		registryLock.RUnlock()
		if !devExists {
			break
		}

		mailboxLock.Lock()
		pendingCmd, hasCmd := commandMailbox[meterName]
		if hasCmd {
			delete(commandMailbox, meterName)
		}
		mailboxLock.Unlock()

		if hasCmd {
			_, _ = port.Write([]byte(pendingCmd + "\r"))
			ansLine, err := reader.ReadString('\r')
			
			registryLock.Lock()
			if err == nil && strings.TrimSpace(ansLine) != "" {
				registry[meterName].LastAns = strings.TrimSpace(ansLine) + "\n"
				registry[meterName].LastUpdate = time.Now()
			} else {
				registry[meterName].LastAns = "NA\n"
			}
			registryLock.Unlock()
		}

		_, _ = port.Write([]byte("I\r"))
		readingLine, err := reader.ReadString('\r')

		registryLock.Lock()
		if err == nil && strings.TrimSpace(readingLine) != "" {
			registry[meterName].LatestCurrent = strings.TrimSpace(readingLine) + "\n"
			registry[meterName].LastUpdate = time.Now()
		} else {
			registry[meterName].LatestCurrent = "NA\n"
			break
		}
		registryLock.Unlock()

		time.Sleep(1 * time.Second)
	}
}

func monitorTopologyLoop() {
	activeWorkers := make(map[string]bool)
	for {
		ports := discoverPorts()
		portsLock.Lock()
		for _, p := range ports {
			if !activeWorkers[p] {
				activeWorkers[p] = true
				go func(portPath string) {
					deviceWorker(portPath)
					portsLock.Lock()
					activeWorkers[portPath] = false
					portsLock.Unlock()
				}(p)
			}
		}
		portsLock.Unlock()
		time.Sleep(5 * time.Second)
	}
}

func encryptPayload(plainBytes []byte) ([]byte, error) {
	k, err := fernet.DecodeKey(psk)
	if err != nil {
		return nil, err
	}
	tok, err := fernet.EncryptAndSign(plainBytes, k)
	return tok, err
}

func decryptPayload(cipherBytes []byte) ([]byte, error) {
	rawKey, err := base64.URLEncoding.DecodeString(psk)
	if err != nil || len(rawKey) != 32 {
		return nil, fmt.Errorf("invalid key config")
	}
	signingKey := rawKey[0:16]
	encryptionKey := rawKey[16:32]

	tokenStr := string(cipherBytes)
	tokenStr = strings.TrimSpace(tokenStr)
	tokBytes, err := base64.URLEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 token")
	}

	if len(tokBytes) < 57 {
		return nil, fmt.Errorf("token too short")
	}

	macOffset := len(tokBytes) - 32
	receivedMac := tokBytes[macOffset:]
	dataToSign := tokBytes[0:macOffset]

	mac := hmac.New(sha256.New, signingKey)
	mac.Write(dataToSign)
	expectedMac := mac.Sum(nil)

	if !hmac.Equal(receivedMac, expectedMac) {
		return nil, fmt.Errorf("hmac validation failed")
	}

	iv := tokBytes[9:25]
	ciphertext := tokBytes[25:macOffset]

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(ciphertext))
	mode.CryptBlocks(decrypted, ciphertext)

	if len(decrypted) == 0 {
		return nil, fmt.Errorf("decryption blank")
	}
	paddingLen := int(decrypted[len(decrypted)-1])
	if paddingLen < 1 || paddingLen > 16 || paddingLen > len(decrypted) {
		return nil, fmt.Errorf("invalid padding")
	}
	
	return decrypted[:len(decrypted)-paddingLen], nil
}

func sendEncryptedResponse(w http.ResponseWriter, statusCode int, rawData []byte) {
	enc, err := encryptPayload(rawData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(statusCode)
	_, _ = w.Write(enc)
}

func handleRouter(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	var pathSegs []string
	for _, p := range parts {
		if p != "" {
			pathSegs = append(pathSegs, p)
		}
	}

	if r.Method == http.MethodGet {
		if len(pathSegs) == 0 {
			res, _ := json.Marshal([]string{"I_METER"})
			sendEncryptedResponse(w, http.StatusOK, res)
			return
		}
		if len(pathSegs) == 1 && pathSegs[0] == "I_METER" {
			registryLock.RLock()
			var names []string
			for k := range registry {
				names = append(names, k)
			}
			registryLock.RUnlock()
			res, _ := json.Marshal(names)
			sendEncryptedResponse(w, http.StatusOK, res)
			return
		}
		if len(pathSegs) == 2 && pathSegs[0] == "I_METER" {
			registryLock.RLock()
			exists := registry[pathSegs[1]] != nil
			registryLock.RUnlock()
			if exists {
				res, _ := json.Marshal([]string{"I", "cmd", "ans", "TLC"})
				sendEncryptedResponse(w, http.StatusOK, res)
				return
			}
		}
		if len(pathSegs) == 3 && pathSegs[0] == "I_METER" {
			meterName := pathSegs[1]
			fileName := pathSegs[2]

			registryLock.RLock()
			info, exists := registry[meterName]
			registryLock.RUnlock()

			if exists {
				var payload string
				if fileName == "I" {
					payload = info.LatestCurrent
				} else if fileName == "ans" {
					registryLock.Lock() // Hold clean lock while reading and writing memory safely
					payload = info.LastAns
					info.LastAns = ""
					registryLock.Unlock()
				} else if fileName == "cmd" {
					payload = "(Write-only channel. Echo your command here)\n"
				} else if fileName == "TLC" {
					tlcSeconds := int(time.Since(info.LastUpdate).Seconds())
					payload = fmt.Sprintf("%d\n", tlcSeconds)
				} else {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				sendEncryptedResponse(w, http.StatusOK, []byte(payload))
				return
			}
		}
	}

	if r.Method == http.MethodPost {
		if len(pathSegs) == 3 && pathSegs[0] == "I_METER" && pathSegs[2] == "cmd" {
			meterName := pathSegs[1]
			encBody, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			decCmd, err := decryptPayload(encBody)
			if err != nil {
				fmt.Println("[-] Rejecting unauthorized POST payload: bad decryption key token")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			mailboxLock.Lock()
			commandMailbox[meterName] = strings.TrimSpace(string(decCmd))
			mailboxLock.Unlock()

			time.Sleep(1200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

func main() {
	go monitorTopologyLoop()
	http.HandleFunc("/", handleRouter)
	fmt.Println("Secure Encrypted Go Server listening on Port 8999...")
	_ = http.ListenAndServe(":8999", nil)
}
