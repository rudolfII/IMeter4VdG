# Variables for easy configuration
PI_USER = ruda
PI_IP = 195.113.22.71
PI_DIR = /home/ruda/mereniI/pi-gateway

# The default rule when you just type 'make'
all: build upload

# Rule to cross-compile the 64-bit binary for the Pi
build:
	@echo "[*] Cross-compiling for 64-bit Raspberry Pi 3..."
	GOOS=linux GOARCH=arm64 go build -o server_pi main.go

# Rule to upload the binary to the Pi using secure copy
upload:
	@echo "[*] Uploading binary to the Pi..."
	scp server_pi $(PI_USER)@$(PI_IP):$(PI_DIR)/
	@echo "[+] Upload complete! File is located at $(PI_DIR)/server_pi"

# Clean up local binaries
clean:
	rm -f server_pi
