import "./style.css";
import { ConnectToPi, SendCommand } from '../wailsjs/go/main/App';

window.connect = async function() {
    let ip = document.getElementById("ipInput").value;
    document.getElementById("statusText").innerText = "Connecting...";
    let result = await ConnectToPi(ip);
    document.getElementById("statusText").innerText = result;
}

window.sendCommand = async function() {
    let cmd = document.getElementById("cmdInput").value;
    let res = await SendCommand(cmd);
    log(`-> Sent: ${cmd} (${res})`);
    document.getElementById("cmdInput").value = "";
}

function log(text) {
    let logArea = document.getElementById("logArea");
    if (logArea) {
        logArea.value += text + "\n";
        logArea.scrollTop = logArea.scrollHeight;
    }
}

// Hook into real-time streams from the Go backend
window.runtime.EventsOn("serial_reply", (data) => {
    log(`<- Reply: ${data.trim()}`);
});

window.runtime.EventsOn("connection_lost", (msg) => {
    document.getElementById("statusText").innerText = "Disconnected";
    log(`!! ${msg}`);
});

