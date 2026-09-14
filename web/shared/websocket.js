(function initializeWebSocket(namespace) {
  const reconnectDelayMS = 2000;

  namespace.connect = function connect(onSnapshot) {
    let reconnectTimer;

    function open() {
      const protocol = location.protocol === 'https:' ? 'wss' : 'ws';
      const socket = new WebSocket(`${protocol}://${location.host}/ws`);

      socket.onmessage = ({data}) => {
        try {
          onSnapshot(JSON.parse(data));
        } catch (error) {
          console.error('Invalid monitor snapshot', error);
        }
      };

      socket.onerror = () => socket.close();
      socket.onclose = () => {
        window.clearTimeout(reconnectTimer);
        reconnectTimer = window.setTimeout(open, reconnectDelayMS);
      };
    }

    open();
  };
}(window.NetworkMonitor = window.NetworkMonitor || {}));
