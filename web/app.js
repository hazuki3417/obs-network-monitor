(function startCombinedPage(namespace) {
  namespace.applyLegacySectionsOption();
  namespace.applyPartsOption();
  const renderLatency = namespace.createLatencyView();
  const renderTraffic = namespace.createTrafficView();
  namespace.connect((snapshot) => {
    renderLatency(snapshot);
    renderTraffic(snapshot);
  });
}(window.NetworkMonitor));
