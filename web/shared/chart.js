(function initializeCharts(namespace) {
  const palette = {
    grid: 'rgba(255, 255, 255, 0.16)',
    muted: '#aeb8c8',
    latency: '#62d9d1',
    transmit: '#ba9cff',
    receive: '#62d9d1',
  };

  function compactNumber(value) {
    if (!Number.isFinite(value)) return '--';
    if (Number.isInteger(value)) return value.toFixed(0);
    return value.toFixed(1).replace(/\.0$/, '');
  }

  function niceMaximum(value, minimum = 10) {
    if (!Number.isFinite(value) || value <= 0) return minimum;
    const padded = value * 1.15;
    const magnitude = 10 ** Math.floor(Math.log10(padded));
    const normalized = padded / magnitude;
    const step = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10;
    return Math.max(minimum, step * magnitude);
  }

  function monotoneControls(points) {
    if (points.length < 2) return [];
    const slopes = points.slice(0, -1).map((point, index) => {
      const next = points[index + 1];
      const width = next.x - point.x;
      return width > 0 ? (next.y - point.y) / width : 0;
    });
    const tangents = new Array(points.length);
    tangents[0] = slopes[0];
    tangents[tangents.length - 1] = slopes[slopes.length - 1];
    for (let index = 1; index < tangents.length - 1; index += 1) {
      const previous = slopes[index - 1];
      const next = slopes[index];
      tangents[index] = previous * next <= 0
        ? 0
        : (2 * previous * next) / (previous + next);
    }
    return points.slice(0, -1).map((point, index) => {
      const next = points[index + 1];
      const width = next.x - point.x;
      return {
        firstX: point.x + width / 3,
        firstY: point.y + (tangents[index] * width) / 3,
        secondX: next.x - width / 3,
        secondY: next.y - (tangents[index + 1] * width) / 3,
      };
    });
  }

  function now() {
    return window.performance && typeof window.performance.now === 'function'
      ? window.performance.now()
      : Date.now();
  }

  function createCanvasChart(canvas, options) {
    const context = canvas.getContext('2d');
    const logicalWidth = options.width;
    const logicalHeight = options.height;
    const plot = {left: 36, right: logicalWidth, top: 1, bottom: logicalHeight - 1};
    const plotWidth = plot.right - plot.left;
    const plotHeight = plot.bottom - plot.top;
    const windowMilliseconds = 60_000;
    let state = {
      history: [],
      maximum: options.minimum,
      hasValues: false,
      anchor: Date.now(),
      receivedAt: now(),
    };

    function resize() {
      const bounds = canvas.getBoundingClientRect();
      const cssWidth = bounds.width || logicalWidth;
      const cssHeight = bounds.height || logicalHeight;
      const ratio = Math.max(1, window.devicePixelRatio || 1);
      const width = Math.round(cssWidth * ratio);
      const height = Math.round(cssHeight * ratio);
      if (canvas.width !== width || canvas.height !== height) {
        canvas.width = width;
        canvas.height = height;
      }
      context.setTransform(width / logicalWidth, 0, 0, height / logicalHeight, 0, 0);
    }

    function drawAxes() {
      context.strokeStyle = palette.grid;
      context.lineWidth = 1;
      context.beginPath();
      [plot.top, plot.top + plotHeight / 2, plot.bottom].forEach((y) => {
        context.moveTo(plot.left, y);
        context.lineTo(plot.right, y);
      });
      context.stroke();

      const maximum = state.hasValues ? compactNumber(state.maximum) : '--';
      const middle = state.hasValues ? compactNumber(state.maximum / 2) : '--';
      context.fillStyle = palette.muted;
      context.font = '700 9px Inter, Segoe UI, sans-serif';
      context.textAlign = 'right';
      context.textBaseline = 'middle';
      context.fillText(maximum, plot.left - 8, 9);
      context.fillText(middle, plot.left - 8, plot.top + plotHeight / 2);
      context.fillText('0', plot.left - 8, plot.bottom - 4);
      context.font = '700 8px Inter, Segoe UI, sans-serif';
      context.fillText(
        state.hasValues ? options.scaleLabel(state.maximum) : options.emptyLabel,
        plot.right - 8,
        12,
      );
    }

    function timestamp(entry, index) {
      const parsed = Date.parse(entry.checkedAt);
      if (Number.isFinite(parsed)) return parsed;
      return state.anchor - (state.history.length - 1 - index) * 1000;
    }

    function segmentsFor(key, visualNow) {
      const segments = [];
      let segment = [];
      state.history.forEach((entry, index) => {
        const value = options.value(entry, key);
        const time = timestamp(entry, index);
        if (!Number.isFinite(value) || time > visualNow + 1000) {
          if (segment.length) segments.push(segment);
          segment = [];
          return;
        }
        const x = plot.right - ((visualNow - time) / windowMilliseconds) * plotWidth;
        const y = plot.bottom - (Math.max(0, value) / state.maximum) * plotHeight;
        segment.push({x, y});
      });
      if (segment.length) segments.push(segment);
      return segments;
    }

    function drawSegment(points) {
      if (points.length === 0) return;
      context.moveTo(points[0].x, points[0].y);
      const controls = monotoneControls(points);
      controls.forEach((control, index) => {
        const next = points[index + 1];
        context.bezierCurveTo(
          control.firstX,
          control.firstY,
          control.secondX,
          control.secondY,
          next.x,
          next.y,
        );
      });
    }

    function drawSeries(visualNow) {
      context.save();
      context.beginPath();
      context.rect(plot.left, plot.top, plotWidth, plotHeight);
      context.clip();
      options.series.forEach((series) => {
        context.beginPath();
        segmentsFor(series.key, visualNow).forEach(drawSegment);
        context.strokeStyle = palette[series.color];
        context.lineWidth = 2;
        context.lineCap = 'round';
        context.lineJoin = 'round';
        context.stroke();
      });
      context.restore();
    }

    function draw(frameNow) {
      resize();
      context.clearRect(0, 0, logicalWidth, logicalHeight);
      drawAxes();
      const visualNow = state.anchor + Math.max(0, frameNow - state.receivedAt);
      drawSeries(visualNow);
    }

    const unsubscribe = namespace.animation.subscribe(draw);
    return {
      update(history, generatedAt) {
        const trimmed = history.slice(-60);
        const values = [];
        trimmed.forEach((entry) => options.series.forEach((series) => {
          const value = options.value(entry, series.key);
          if (Number.isFinite(value)) values.push(value);
        }));
        const parsedAnchor = Date.parse(generatedAt);
        state = {
          history: trimmed,
          maximum: niceMaximum(Math.max(0, ...values), options.minimum),
          hasValues: values.length > 0,
          anchor: Number.isFinite(parsedAnchor) ? parsedAnchor : Date.now(),
          receivedAt: now(),
        };
      },
      destroy: unsubscribe,
    };
  }

  namespace.charts = {compactNumber, niceMaximum, monotoneControls, createCanvasChart};
}(window.NetworkMonitor = window.NetworkMonitor || {}));
