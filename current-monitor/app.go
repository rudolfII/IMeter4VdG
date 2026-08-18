package main

import (
	"context"
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
	"strconv"
	"strings"
	"time"

	"github.com/fernet/fernet-go"
)

type App struct {
	ctx context.Context
}

type DeviceInfo struct {
	LatestCurrent   float64 `json:"latest_current"`
	Unit            string  `json:"unit"`
	LastRawResponse string  `json:"last_raw_response"`
	LastUpdate      int64   `json:"last_update"`
	DeviceID        string  `json:"device_id"`
}

const psk = "9AiX1wUIPnZYmaVDkCFI8c4nikAne-cZgfGGd_BdOA4="

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func resolveTargetUrl(input, suffixPath string) string {
	input = strings.TrimSpace(input)
	if strings.Contains(input, ":") {
		return fmt.Sprintf("http://%s%s", input, suffixPath)
	}
	return fmt.Sprintf("http://%s:8999%s", input, suffixPath)
}

func (a *App) decryptResponse(cipherBytes []byte) ([]byte, error) {
	rawKey, err := base64.URLEncoding.DecodeString(psk)
	if err != nil || len(rawKey) != 32 {
		return nil, fmt.Errorf("invalid key config")
	}
	signingKey := rawKey[0:16]
	encryptionKey := rawKey[16:32]

	tokenStr := strings.TrimSpace(string(cipherBytes))
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
	if !hmac.Equal(receivedMac, mac.Sum(nil)) {
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

func (a *App) encryptRequest(plainBytes []byte) ([]byte, error) {
	k, err := fernet.DecodeKey(psk)
	if err != nil {
		return nil, err
	}
	return fernet.EncryptAndSign(plainBytes, k)
}

func (a *App) FetchDeviceList(ipAddress string) ([]string, error) {
	url := resolveTargetUrl(ipAddress, "/I_METER")

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	decrypted, err := a.decryptResponse(body)
	if err != nil {
		return nil, err
	}

	var devices []string
	if err := json.Unmarshal(decrypted, &devices); err != nil {
		return nil, err
	}

	var fullPaths []string
	for _, d := range devices {
		fullPaths = append(fullPaths, "/dev/"+d)
	}
	return fullPaths, nil
}

func (a *App) FetchMeterData(ipAddress string, portPath string) (DeviceInfo, error) {
	meterName := filepath.Base(portPath)
	baseUrl := resolveTargetUrl(ipAddress, fmt.Sprintf("/I_METER/%s", meterName))

	var info DeviceInfo
	info.DeviceID = meterName
	info.LastUpdate = -1 

	if resp, err := http.Get(baseUrl + "/I"); err == nil {
		if body, rErr := io.ReadAll(resp.Body); rErr == nil {
			if dec, dErr := a.decryptResponse(body); dErr == nil {
				rawStr := strings.TrimSpace(string(dec))
				
				info.Unit = "nA"
				if strings.Contains(rawStr, "uA") {
					info.Unit = "uA"
				}

				fields := strings.Fields(rawStr)
				if len(fields) >= 3 && fields[0] == "I" {
					if val, pErr := strconv.ParseFloat(fields[2], 64); pErr == nil {
						info.LatestCurrent = val
						info.LastUpdate = time.Now().Unix()
					}
				}
			}
		}
		resp.Body.Close()
	}

	if resp, err := http.Get(baseUrl + "/ans"); err == nil {
		if body, rErr := io.ReadAll(resp.Body); rErr == nil {
			if dec, dErr := a.decryptResponse(body); dErr == nil {
				rawStr := strings.TrimSpace(string(dec))
				if rawStr != "No answer yet" && rawStr != "NA" && rawStr != "" {
					info.LastRawResponse = rawStr
				} else {
					info.LastRawResponse = ""
				}
			}
		}
		resp.Body.Close()
	}

	return info, nil
}

func (a *App) SendCustomCommand(ipAddress string, portPath string, commandStr string) (string, error) {
	meterName := filepath.Base(portPath)
	url := resolveTargetUrl(ipAddress, fmt.Sprintf("/I_METER/%s/cmd", meterName))

	encCmd, err := a.encryptRequest([]byte(commandStr))
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "text/plain", strings.NewReader(string(encCmd)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server rejected command with code: %d", resp.StatusCode)
	}

	return "Success", nil
}
