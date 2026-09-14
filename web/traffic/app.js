(function startTrafficPage(namespace) {
  namespace.applyPartsOption();
  namespace.connect(namespace.createTrafficView());
}(window.NetworkMonitor));
