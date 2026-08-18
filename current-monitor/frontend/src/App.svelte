<script>
  import { onMount, onDestroy } from 'svelte';
  import { FetchDeviceList, FetchMeterData, SendCustomCommand } from '../wailsjs/go/main/App';
  import { WindowSetSize } from '../wailsjs/runtime/runtime';

  let piIpAddress = $state("195.113.22.71:2000"); 
  let activeMeters = $state([]); 
  let currentTime = $state(Math.floor(Date.now() / 1000)); 
  let layoutContainerRef = $state(null);
  let resizeThrottleTimeout = null;
  let lastSeenDeviceCount = 0;
  let intervalId;
  let timeTickerId;

  const MAX_LOG_ROWS = 100;

  const segmentMap = {
    '0': [true,  true,  true,  true,  true,  true,  false],
    '1': [false, true,  true,  false, false, false, false],
    '2': [true,  true,  false, true,  true,  false, true ],
    '3': [true,  true,  true,  true,  false, false, true ],
    '4': [false, true,  true,  false, false, true,  true ],
    '5': [true,  false, true,  true,  false, true,  true ],
    '6': [true,  false, true,  true,  true,  true,  true ],
    '7': [true,  true,  true,  false, false, false, false],
    '8': [true,  true,  true,  true,  true,  true,  true ],
    '9': [true,  true,  true,  true,  false, true,  true ],
    '-': [false, false, false, false, false, false, true ],
    ' ': [false, false, false, false, false, false, false]
  };

  function isSegOn(char, idx) { 
    return (segmentMap[String(char)] || segmentMap[' '])[idx] === true; 
  }

  function parseDigits(numValue, isInitialized) {
    if (!isInitialized) {
      return [{ char: '-', decimal: false }, { char: '-', decimal: false }, { char: '-', decimal: false }, { char: '-', decimal: false }];
    }
    if (typeof numValue !== 'number' || isNaN(numValue)) {
      return Array(4).fill({ char: ' ', decimal: false });
    }
    let str = numValue.toFixed(2).padStart(5, '0');
    let res = [], chars = str.split('');
    for (let i = 0; i < chars.length; i++) {
      if (chars[i] === '.') continue;
      res.push({ char: chars[i], decimal: (chars[i+1] === '.') });
    }
    return res.slice(-4);
  }

  async function syncNetworkTopology() {
    try {
      const ports = await FetchDeviceList(piIpAddress);
      if (!ports || ports.length === 0) { activeMeters = []; return; }
      let updatedMeters = [];

      for (let port of ports) {
        let existing = activeMeters.find(m => m.port === port);
        let currentTextVal = 0.00, unitLabel = "nA"; 
        let currentLogs = existing ? [...existing.logs] : [];
        let inputBuffer = existing ? existing.commandInput : "";
        let elementRef = existing?.terminalRef || null;

        let lastSeenRawTs = existing ? existing.lastSeenRawTs : -1;
        let localLaptopWatchdogTs = existing ? existing.localLaptopWatchdogTs : Math.floor(Date.now() / 1000);
        
        // PURE DEFAULT CHECK: If lastSeenRawTs is -1, freeze display text as "--"
        let clockDisplayStr = existing ? existing.clockDisplayStr : '--';

        let rawPortName = port.replace("/dev/", "").toUpperCase();
        let displayIDName = rawPortName; 

        try {
          const state = await FetchMeterData(piIpAddress, port);
          currentTextVal = state.latest_current;
          unitLabel = state.unit === "uA" ? "μA" : "nA";
          
          if (state.device_id && state.device_id.trim() !== "") {
            displayIDName = state.device_id;
          }
          if (state.last_update && state.last_update > 0 && state.last_update !== lastSeenRawTs) {
            lastSeenRawTs = state.last_update;
            localLaptopWatchdogTs = Math.floor(Date.now() / 1000); 
            clockDisplayStr = new Date().toLocaleTimeString();     
          } else if (lastSeenRawTs === -1) {
            clockDisplayStr = '--';
          }
          if (state.last_raw_response) {
            currentLogs.push("[" + new Date().toLocaleTimeString() + "] " + state.last_raw_response);
            if (currentLogs.length > MAX_LOG_ROWS) currentLogs.shift();
            if (elementRef) {
              setTimeout(() => { elementRef.scrollTop = elementRef.scrollHeight; }, 40);
            }
          }
        } catch (e) {
          currentTextVal = 0.00;
          localLaptopWatchdogTs = Math.floor(Date.now() / 1000);
        }

        updatedMeters.push({
          port, displayName: displayIDName, sysComment: rawPortName,    
          value: currentTextVal, unit: unitLabel, logs: currentLogs, commandInput: inputBuffer,
          terminalRef: elementRef, lastSeenRawTs, localLaptopWatchdogTs, clockDisplayStr
        });
      }
      activeMeters = updatedMeters;
    } catch (err) {
      activeMeters = [];
    }
  }

  async function executeCommand(event, meter) {
    event.preventDefault();
    if (!meter.commandInput) return;
    const cmd = meter.commandInput;
    meter.commandInput = ""; 
    try {
      meter.logs.push("[" + new Date().toLocaleTimeString() + "] Outbound: '" + cmd + "'");
      if (meter.logs.length > MAX_LOG_ROWS) meter.logs.shift();
      if (meter.terminalRef) setTimeout(() => { meter.terminalRef.scrollTop = meter.terminalRef.scrollHeight; }, 40);
      await SendCustomCommand(piIpAddress, meter.port, cmd);
    } catch (e) {
      meter.logs.push("[Error]: " + e);
    }
  }

  $effect(() => {
    if (activeMeters && layoutContainerRef) {
      if (activeMeters.length === lastSeenDeviceCount) return;
      lastSeenDeviceCount = activeMeters.length;
      if (resizeThrottleTimeout) clearTimeout(resizeThrottleTimeout);
      resizeThrottleTimeout = setTimeout(() => {
        WindowSetSize(Math.max(300, layoutContainerRef.scrollWidth + 60), Math.max(420, layoutContainerRef.scrollHeight + 60));
        resizeThrottleTimeout = null;
      }, 250);
    }
  });

  onMount(() => {
    syncNetworkTopology();
    intervalId = setInterval(syncNetworkTopology, 1000);
    timeTickerId = setInterval(() => { currentTime = Math.floor(Date.now() / 1000); }, 500);
  });

  onDestroy(() => {
    clearInterval(intervalId);
    clearInterval(timeTickerId);
  });
</script>

<main class="container">
  <div class="dashboard-layout" bind:this={layoutContainerRef}>
    <div class="main-content-area">
      <div class="meter-grid">
        {#each activeMeters as meter}
          <div class="monitor-card">
            
            <div class="title-header-group">
              <h2 class="main-id-title">{meter.displayName}</h2>
              <span class="system-port-comment">({meter.sysComment}{#if meter.lastSeenRawTs !== -1} &bull; {meter.unit}{/if})</span>
            </div>
            
            <div class="hardware-bezel">
              <div class="display-panel">
                {#each parseDigits(meter.value, meter.lastSeenRawTs !== -1) as digit}
                  <svg class="digit-svg" viewBox="0 0 30 46" width="40" height="66">
                    <g transform="skewX(-10) translate(2.5, 2)">
                      <polygon points="1,0 23,0 18.5,4.5 5.5,4.5" class="seg" class:on={isSegOn(digit.char, 0)} />
                      <polygon points="0,0.8 4.5,5.3 4.5,18.7 0,21.2" class="seg" class:on={isSegOn(digit.char, 5)} />
                      <polygon points="24,0.8 24,21.2 19.5,18.7 19.5,5.3" class="seg" class:on={isSegOn(digit.char, 1)} />
                      <polygon points="1,22 4.5,19.75 19.5,19.75 23,22 19.5,24.25 4.5,24.25" class="seg" class:on={isSegOn(digit.char, 6)} />
                      <polygon points="0,22.8 4.5,25.3 4.5,38.7 0,43.2" class="seg" class:on={isSegOn(digit.char, 4)} />
                      <polygon points="24,22.8 24,43.2 19.5,38.7 19.5,25.3" class="seg" class:on={isSegOn(digit.char, 2)} />
                      <polygon points="5.5,39.5 18.5,39.5 23,44 1,44" class="seg" class:on={isSegOn(digit.char, 3)} />
                      <circle cx="29" cy="42.5" r="2.2" class="decimal-dot" class:on={digit.decimal} />
                    </g>
                  </svg>
                {/each}
              </div>
            </div>

            <!-- WATCHDOG CHECK: If lastSeenRawTs is -1, force red state immediately -->
            <div class="heartbeat-tracker" class:stale={meter.lastSeenRawTs === -1 || (currentTime - meter.localLaptopWatchdogTs > 3)}>
              Latest: {meter.clockDisplayStr}
            </div>

            <form class="command-tray" onsubmit={(e) => executeCommand(e, meter)}>
              <input type="text" bind:value={meter.commandInput} placeholder="Send command..." />
              <button type="submit">Send</button>
            </form>

            <div class="activity-terminal" bind:this={meter.terminalRef}>
              {#each meter.logs as logRow}
                <div class="log-line">{logRow}</div>
              {:else}
                <span class="placeholder-text">No asynchronous event updates.</span>
              {/each}
            </div>

          </div>
        {:else}
          <div class="empty-state">Awaiting dynamic peripheral discovery connections over network topology...</div>
        {/each}
      </div>
    </div>

    <footer class="network-config-footer">
      <div class="network-config">
        <label for="ip-box">IP:</label>
        <input id="ip-box" type="text" bind:value={piIpAddress} placeholder="e.g. 192.168.1.5" />
      </div>
    </footer>

  </div>
</main>

<style>
  :global(html), :global(body), :global(#app) {
    display: block !important;
    width: 100% !important;
    height: 100% !important;
    margin: 0 !important;
    padding: 0 !important;
    overflow: hidden !important;
    background-color: #e6e6e8 !important;
  }
  .container {
    display: block !important;
    width: 100% !important;
    min-width: 100% !important;
    min-height: 100vh;
    box-sizing: border-box;
    padding: 20px;
    overflow: hidden !important;
  }
  .dashboard-layout {
    display: flex;
    flex-direction: column;
    align-items: flex-start;     
    justify-content: flex-start;
    gap: 20px;
    width: max-content;
    max-width: none !important;
  }
  .main-content-area {
    width: 100%;
    display: flex;
    justify-content: flex-start; 
    align-items: flex-start;
  }
  .meter-grid {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-start; 
    gap: 20px;
    width: 100%;
  }
  .monitor-card {
    background: #f0f0f2;
    border: 1px solid #bcbcbe;
    border-radius: 6px;
    padding: 20px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    box-shadow: 0 4px 10px rgba(0,0,0,0.05);
    width: 260px; 
    box-sizing: border-box;
    flex-shrink: 0; 
  }
  .title-header-group {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    border-bottom: 1px dashed #bcbcbe;
    width: 100%;
    padding-bottom: 6px;
    box-sizing: border-box;
  }
  .main-id-title {
    color: #111;
    font-size: 1.05rem; 
    font-weight: 800;
    letter-spacing: 0.02em;
    margin: 0;
    text-align: center;
    word-break: break-word;
  }
  .system-port-comment {
    color: #66666e;     
    font-size: 0.72rem; 
    font-family: monospace;
    font-weight: normal;
  }
  .hardware-bezel {
    background: #000000; 
    border: 3px solid #7a7a7c; 
    border-radius: 2px;
    padding: 6px 12px; 
    box-shadow: inset 0px 0px 6px rgba(0,0,0,0.95); 
    display: inline-flex;
  }
  .display-panel { 
    display: flex; 
    gap: 5px; 
    background: #000000; 
  }
  .digit-svg { 
    overflow: visible; 
  }
  .seg { 
    fill: #120202; 
    stroke: #000000; 
    stroke-width: 0.4px; 
    transition: fill 0.01s ease; 
  }
  .seg.on { 
    fill: #ff1212; 
  }
  .decimal-dot { 
    fill: #120202; 
    stroke: #000000; 
    stroke-width: 0.4px; 
  }
  .decimal-dot.on { 
    fill: #ff1212; 
  }
  .heartbeat-tracker {
    font-size: 0.85rem;
    font-weight: bold;
    color: #10b981; 
    font-family: monospace;
    transition: color 0.25s ease;
  }
  .heartbeat-tracker.stale {
    color: #ef4444; 
  }
  .activity-terminal {
    width: 100%; 
    background: #1c1c1e; 
    border: 1px solid #2c2c2e; 
    border-radius: 4px;
    padding: 6px 10px; 
    color: #34c759; 
    font-family: monospace; 
    font-size: 0.75rem;
    text-align: left; 
    box-sizing: border-box; 
    display: flex; 
    flex-direction: column; 
    gap: 2px;
    height: 85px; 
    overflow-y: auto; 
  }
  .log-line {
    word-break: break-all;
    line-height: 1.1;
  }
  .placeholder-text {
    color: #55555c;
    font-style: italic;
  }
  .command-tray { 
    display: flex; 
    gap: 6px; 
    width: 100%; 
  }
  .command-tray input { 
    width: calc(100% - 68px);
    padding: 6px; 
    border: 1px solid #bcbcbe; 
    border-radius: 4px; 
    font-size: 0.8rem; 
    outline: none; 
    background: #fff; 
    box-sizing: border-box;
  }
  .command-tray button { 
    width: 62px;
    padding: 6px 0; 
    background: #7a7a7c; 
    border: 1px solid #bcbcbe; 
    color: white; 
    border-radius: 4px; 
    cursor: pointer; 
    text-align: center;
    box-sizing: border-box;
  }
  .command-tray button:hover { 
    background: #4a4a4c; 
  }
  .network-config-footer {
    width: 100%;
    display: flex;
    justify-content: center;
    margin-top: 40px;
    padding-top: 8px;
    border-top: 1px dashed #bcbcbe;
  }
  .network-config {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    font-size: 0.85rem;
    color: #444;
    white-space: nowrap; 
    flex-wrap: wrap;    
  }
  .network-config label {
    font-weight: bold;
    flex-shrink: 0;
  }
  .network-config input { 
    padding: 4px 6px; 
    border: 1px solid #bcbcbe; 
    border-radius: 4px; 
    text-align: center; 
    width: 130px; 
    outline: none;
    background: #fff;
  }
  .empty-state {
    font-size: 0.85rem;
    color: #666;
    font-style: italic;
    margin-top: 20px;
    text-align: center;
    width: 240px;
    white-space: normal;
    line-height: 1.3;
  }
</style>
