(function startHomePage(namespace) {
  const websocketStatus = document.querySelector('#websocket-status');
  const streamStatus = document.querySelector('#stream-status');
  const lastUpdate = document.querySelector('#last-update');
  const shutdownButton = document.querySelector('#shutdown-button');
  const shutdownStatus = document.querySelector('#shutdown-status');

  function setStatus(element, text, state) {
    element.lastChild.textContent = text;
    element.className = `status is-${state}`;
  }

  shutdownButton.addEventListener('click', async () => {
    if (!window.confirm('OBS Network Monitorを終了しますか？')) return;

    shutdownButton.disabled = true;
    shutdownStatus.textContent = 'Shutting down...';
    try {
      const response = await window.fetch('/api/shutdown', {
        method: 'POST',
        headers: {'X-OBS-Network-Monitor-Shutdown': '1'},
      });
      if (!response.ok) throw new Error(`shutdown failed: ${response.status}`);
      shutdownStatus.textContent = 'Stopped — this page can be closed.';
    } catch {
      shutdownStatus.textContent = '終了要求を送信できませんでした。';
      shutdownButton.disabled = false;
    }
  });

  namespace.connect(
    (snapshot) => {
      setStatus(streamStatus, 'Receiving data', 'healthy');
      const generatedAt = new Date(snapshot.generatedAt);
      lastUpdate.textContent = Number.isNaN(generatedAt.getTime())
        ? 'Received now'
        : generatedAt.toLocaleString('ja-JP');
    },
    (state) => {
      if (state === 'connected') {
        setStatus(websocketStatus, 'Connected', 'healthy');
        return;
      }
      if (state === 'reconnecting') {
        setStatus(websocketStatus, 'Reconnecting', 'error');
        setStatus(streamStatus, 'Waiting for data', 'pending');
        return;
      }
      setStatus(websocketStatus, 'Connecting', 'pending');
    },
  );
}(window.NetworkMonitor));
