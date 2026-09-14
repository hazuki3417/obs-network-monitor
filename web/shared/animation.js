(function initializeAnimation(namespace) {
  const callbacks = new Set();
  const frameInterval = 1000 / 60;
  let frameRequest = null;
  let lastFrame = -Infinity;

  function frame(now) {
    frameRequest = null;
    if (now - lastFrame >= frameInterval - 0.5) {
      lastFrame = now;
      callbacks.forEach((callback) => callback(now));
    }
    if (callbacks.size > 0) frameRequest = window.requestAnimationFrame(frame);
  }

  function subscribe(callback) {
    callbacks.add(callback);
    if (frameRequest === null) frameRequest = window.requestAnimationFrame(frame);
    return function unsubscribe() {
      callbacks.delete(callback);
      if (callbacks.size === 0 && frameRequest !== null) {
        window.cancelAnimationFrame(frameRequest);
        frameRequest = null;
        lastFrame = -Infinity;
      }
    };
  }

  namespace.animation = {subscribe};
}(window.NetworkMonitor = window.NetworkMonitor || {}));
