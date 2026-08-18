#!/usr/bin/env python3
# /// script
# dependencies = [
#     "fusepy",
#     "cryptography",
# ]
# ///
import sys
import os
import stat
import urllib.request
import json
import subprocess
import time
import traceback
from fuse import FUSE, Operations, LoggingMixIn
from cryptography.fernet import Fernet

# Hardcoded Pre-Shared Testing Key
PRE_SHARED_KEY = b"9AiX1wUIPnZYmaVDkCFI8c4nikAne-cZgfGGd_BdOA4="
cipher = Fernet(PRE_SHARED_KEY)

class RaspberryDeviceFS(LoggingMixIn, Operations):
    def __init__(self, pi_ip):
        self.base_url = f"http://{pi_ip}:8999"
        self.server_online = True
        self.last_retry_time = 0
        self.seen_meters = set()

    def _api_request(self, path, data=None):
        current_time = time.time()

        if not self.server_online:
            if current_time - self.last_retry_time < 5.0:
                return self._get_fallback_payload(path)
            self.last_retry_time = current_time

        try:
            if data is not None:
                data = cipher.encrypt(data)

            req = urllib.request.Request(f"{self.base_url}{path}", data=data)
            with urllib.request.urlopen(req, timeout=0.8) as response:
                raw_response = response.read()
                
                if not self.server_online:
                    print("[+] CONNECTION RESTORED: Raspberry Pi hardware server is back online.")
                    self.server_online = True

                if data is None and raw_response:
                    try:
                        return cipher.decrypt(raw_response)
                    except Exception:
                        return b"ERROR: Invalid Pre-Shared Key Configuration\n"
                return raw_response

        except Exception:
            if self.server_online:
                print("[-] CONNECTION LOST: Raspberry Pi server went offline. Engaging circuit breaker...")
                self.server_online = False
                self.last_retry_time = current_time
            
            return self._get_fallback_payload(path)

    def _get_fallback_payload(self, path):
        if path == "/":
            return b"[\"I_METER\"]"
        if path == "/I_METER":
            return b"[]"
        return b"NA\n"

    def getattr(self, path, fh=None):
        try:
            parts = [p for p in path.split("/") if p]

            if path == '/':
                return dict(st_mode=(stat.S_IFDIR | 0o755), st_nlink=2)
            if len(parts) == 1 and parts[0] == 'I_METER':
                return dict(st_mode=(stat.S_IFDIR | 0o755), st_nlink=2)
            if len(parts) == 2 and parts[0] == 'I_METER':
                meter_name = parts[1]
                if meter_name not in self.seen_meters:
                    self.seen_meters.add(meter_name)
                    print(f"[+] Mount-Discovery: Found meter '{meter_name}' on remote network loop.")
                return dict(st_mode=(stat.S_IFDIR | 0o755), st_nlink=2)
            if len(parts) == 3 and parts[0] == 'I_METER' and parts[2] in ['I', 'cmd', 'ans', 'TLC']:
                content = self._api_request(path)
                return dict(st_mode=(stat.S_IFREG | 0o666), st_nlink=1, st_size=len(content))
            
            raise OSError(2) # ENOENT (No such file or directory)
        except Exception as e:
            print(f"[-] Error in getattr for path {path}: {e}")
            traceback.print_exc()
            raise OSError(2)

    def readdir(self, path, fh):
        try:
            raw_entries = self._api_request(path)
            entries = ['.', '..']
            if raw_entries:
                try:
                    if not raw_entries.startswith(b"ERROR:"):
                        decoded_list = json.loads(raw_entries.decode())
                        entries.extend(decoded_list)
                        
                        if path == "/I_METER":
                            for meter_name in decoded_list:
                                if meter_name not in self.seen_meters:
                                    self.seen_meters.add(meter_name)
                                    print(f"[+] Mount-Discovery: Found meter '{meter_name}' on remote network loop.")
                except Exception as e:
                    print(f"[-] Error parsing readdir response: {e}")
            return entries
        except Exception as e:
            print(f"[-] Error in readdir for path {path}: {e}")
            return ['.', '..']

    def read(self, path, size, offset, fh):
        try:
            content = self._api_request(path)
            return content[offset:offset + size]
        except Exception as e:
            print(f"[-] Error in read for path {path}: {e}")
            return b""

    def write(self, path, data, offset, fh):
        try:
            parts = [p for p in path.split("/") if p]
            if len(parts) == 3 and parts[0] == 'I_METER' and parts[2] == 'cmd':
                self._api_request(path, data=data)
                return len(data)
            return 0
        except Exception as e:
            print(f"[-] Error in write for path {path}: {e}")
            return 0

    def truncate(self, path, length, fh=None): return 0
    def open(self, path, fi_flags): return 0
    def flush(self, path, fh): return 0

if __name__ == '__main__':
    if len(sys.argv) != 3:
        print('Usage: uv run client_mount.py <RaspberryPi_IP> <Local_Mountpoint>')
        sys.exit(1)
        
    pi_ip_address = sys.argv[1]
    mountpoint = os.path.abspath(sys.argv[2])
    
    if os.path.exists(mountpoint):
        print(f"[*] Sanitising mountpoint target: {mountpoint}...")
        subprocess.run(["fusermount3", "-uz", mountpoint], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(0.2)
    else:
        os.makedirs(mountpoint, exist_ok=True)

    print(f"[+] Initialising secure mount bridge onto {mountpoint}...")
    try:
        FUSE(
            RaspberryDeviceFS(pi_ip_address), 
            mountpoint, 
            nothreads=True, 
            foreground=True,
            attr_timeout=0.0,
            entry_timeout=0.0,
            negative_timeout=0.0
        )
    except Exception as e:
        print(f"[-] FUSE loop terminated: {e}")
