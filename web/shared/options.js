(function initializeDisplayOptions(namespace) {
  function selection(search, key, allowed) {
    const requested = new URLSearchParams(search).get(key) || '';
    const values = new Set(requested
      .split(',')
      .map((value) => value.trim().toLowerCase())
      .filter((value) => allowed.includes(value)));
    return values.size === 1 ? [...values][0] : 'all';
  }

  namespace.applyPartsOption = function applyPartsOption(search = location.search) {
    document.body.dataset.parts = selection(search, 'parts', ['values', 'graph']);
  };
}(window.NetworkMonitor = window.NetworkMonitor || {}));
