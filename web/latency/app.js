(function startLatencyPage(namespace) {
  namespace.applyPartsOption();
  namespace.connect(namespace.createLatencyView());
}(window.NetworkMonitor));
