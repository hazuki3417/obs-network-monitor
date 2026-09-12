const statusEl = document.querySelector('#status');
const latencyEl = document.querySelector('#latency');
const updatedEl = document.querySelector('#updated');

function connect() {
  const protocol = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(`${protocol}://${location.host}/ws`);

  ws.onmessage = ({data}) => {
    const s = JSON.parse(data);
    statusEl.textContent = s.online ? 'ONLINE' : 'OFFLINE';
    statusEl.className = s.online ? 'online' : 'offline';
    latencyEl.textContent = `${s.latencyMs} ms`;
    updatedEl.textContent = new Date(s.checkedAt).toLocaleTimeString();
  };

  ws.onclose = () => {
    statusEl.textContent = 'DISCONNECTED';
    statusEl.className = 'offline';
    setTimeout(connect, 2000);
  };
}

connect();
