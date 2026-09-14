(function initializeDom(namespace) {
  function updateText(element, value) {
    const text = String(value);
    if (element.textContent === text) return;
    element.textContent = text;
    if (typeof element.animate !== 'function') return;
    element.getAnimations?.().forEach((animation) => animation.cancel());
    element.animate(
      [{opacity: 0.55}, {opacity: 1}],
      {duration: 150, easing: 'ease-out'},
    );
  }

  namespace.dom = {updateText};
}(window.NetworkMonitor = window.NetworkMonitor || {}));
