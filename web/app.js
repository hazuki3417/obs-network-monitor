(function startHomePage(namespace) {
  const websocketStatus = document.querySelector('#websocket-status');
  const streamStatus = document.querySelector('#stream-status');
  const lastUpdate = document.querySelector('#last-update');

  function setStatus(element, text, state) {
    element.lastChild.textContent = text;
    element.className = `status is-${state}`;
  }

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
