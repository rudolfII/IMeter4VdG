package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx  context.Context
	conn net.Conn
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ConnectToPi handles the TCP socket connection to your ser2net bridge on port 2000
func (a *App) ConnectToPi(ip string) string {
	if a.conn != nil {
		a.conn.Close()
	}

	address := fmt.Sprintf("%s:2000", ip)
	var err error
	
	// Open a raw TCP link with a 4-second timeout limit
	a.conn, err = net.DialTimeout("tcp", address, 4*time.Second)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// Spin off an asynchronous background loop to listen for your device replies
	go a.listenForReplies()

	return "Connected successfully!"
}

// SendCommand pipes your GUI text input straight to the Raspberry Pi relay
func (a *App) SendCommand(cmd string) string {
	if a.conn == nil {
		return "Error: Not connected to Raspberry Pi"
	}
	// Append the Carriage Return + Newline (\r\n) that your serial hardware needs
	_, err := fmt.Fprintf(a.conn, "%s\r\n", cmd)
	if err != nil {
		return fmt.Sprintf("Failed to send: %v", err)
	}
	return "Command sent"
}

// Background thread reading returning data packets from the Pi's serial link
func (a *App) listenForReplies() {
	reader := bufio.NewReader(a.conn)
	for {
		reply, err := reader.ReadString('\n')
		if err != nil {
			// Notify the Javascript user interface that the link went down
			runtime.EventsEmit(a.ctx, "connection_lost", "Disconnected from server")
			a.conn = nil
			return
		}
		// Stream the real-time incoming string straight to the visual layout screen
		runtime.EventsEmit(a.ctx, "serial_reply", reply)
	}
}

