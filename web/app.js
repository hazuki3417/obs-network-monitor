(function startCombinedPage(namespace) {
  namespace.applyLegacySectionsOption();
  namespace.applyPartsOption();
  const sections = document.body.dataset.sections;
  const renderLatency = sections === 'traffic' ? null : namespace.createLatencyView();
  const renderTraffic = sections === 'latency' ? null : namespace.createTrafficView();
  namespace.connect((snapshot) => {
    renderLatency?.(snapshot);
    renderTraffic?.(snapshot);
  });
}(window.NetworkMonitor));
