(function initializeDom(namespace) {
  function updateText(element, value) {
    const text = String(value);
    if (element.textContent === text) return;
    element.textContent = text;
  }

  namespace.dom = {updateText};
}(window.NetworkMonitor = window.NetworkMonitor || {}));
