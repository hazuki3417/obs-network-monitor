(function initializeCharts(namespace) {
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

  function monotonePath(points) {
    if (points.length === 0) return '';
    if (points.length === 1) {
      return `M ${points[0].x.toFixed(2)} ${points[0].y.toFixed(2)}`;
    }

    const slopes = points.slice(0, -1).map((point, index) => {
      const next = points[index + 1];
      return (next.y - point.y) / (next.x - point.x);
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

    let path = `M ${points[0].x.toFixed(2)} ${points[0].y.toFixed(2)}`;
    for (let index = 0; index < points.length - 1; index += 1) {
      const point = points[index];
      const next = points[index + 1];
      const width = next.x - point.x;
      const firstControlX = point.x + width / 3;
      const firstControlY = point.y + (tangents[index] * width) / 3;
      const secondControlX = next.x - width / 3;
      const secondControlY = next.y - (tangents[index + 1] * width) / 3;
      path += ` C ${firstControlX.toFixed(2)} ${firstControlY.toFixed(2)}`
        + ` ${secondControlX.toFixed(2)} ${secondControlY.toFixed(2)}`
        + ` ${next.x.toFixed(2)} ${next.y.toFixed(2)}`;
    }
    return path;
  }

  function segmentedSmoothPath(history, pointForEntry) {
    const segments = [];
    let segment = [];
    history.forEach((entry, index) => {
      const point = pointForEntry(entry, index);
      if (point) {
        segment.push(point);
        return;
      }
      if (segment.length) segments.push(segment);
      segment = [];
    });
    if (segment.length) segments.push(segment);
    return segments.map(monotonePath).join(' ');
  }

  namespace.charts = {
    compactNumber,
    niceMaximum,
    monotonePath,
    segmentedSmoothPath,
  };
}(window.NetworkMonitor = window.NetworkMonitor || {}));
