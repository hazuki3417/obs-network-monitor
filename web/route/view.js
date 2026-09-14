(function initializeRouteView(namespace) {
  const latencyJumpThreshold = 20;

  function rttText(value) {
    return Number.isFinite(value) ? `${Math.round(value)} ms` : '--';
  }

  function exactNode(hop, kind = 'hop') {
    const anomaly = Number.isFinite(hop.deltaRttMs) && hop.deltaRttMs >= latencyJumpThreshold;
    return {
      kind: anomaly ? (kind === 'target' ? 'target-latency' : 'latency')
        : !hop.responded ? 'no-response' : kind,
      label: kind === 'local' ? 'LOCAL' : kind === 'target' ? 'TARGET' : `HOP ${String(hop.number).padStart(2, '0')}`,
      rtt: rttText(hop.rttMs),
      detail: !hop.responded
        ? 'NO RESPONSE'
        : anomaly ? `+${Math.round(hop.deltaRttMs)} ms  LATENCY` : '',
      number: hop.number,
      delta: anomaly ? hop.deltaRttMs : 0,
    };
  }

  function groupedNode(hops) {
    const last = hops[hops.length - 1];
    return {
      kind: 'group',
      label: `HOP ×${hops.length}`,
      rtt: last && last.responded ? rttText(last.rttMs) : '--',
      detail: '',
      number: last ? last.number : 0,
      delta: 0,
    };
  }

  function nodeCount(middle, selected) {
    let groups = 0;
    let inGroup = false;
    middle.forEach((hop) => {
      if (selected.has(hop.number)) {
        inGroup = false;
        return;
      }
      if (!inGroup) groups += 1;
      inGroup = true;
    });
    return 2 + selected.size + groups;
  }

  function selectImportant(middle, maxNodes) {
    const candidates = middle.filter((hop) => (
      !hop.responded || (Number.isFinite(hop.deltaRttMs) && hop.deltaRttMs >= latencyJumpThreshold)
    )).sort((left, right) => {
      const leftPriority = left.responded ? left.deltaRttMs : -1;
      const rightPriority = right.responded ? right.deltaRttMs : -1;
      return rightPriority - leftPriority;
    });
    const selected = new Set();
    candidates.forEach((hop) => {
      const candidate = new Set(selected);
      candidate.add(hop.number);
      if (nodeCount(middle, candidate) <= maxNodes) selected.add(hop.number);
    });
    return selected;
  }

  function mergeConsecutiveLatency(nodes) {
    const merged = [];
    nodes.forEach((node) => {
      const previous = merged[merged.length - 1];
      if (previous && (previous.kind === 'latency' || previous.kind === 'latency-group')
        && node.kind === 'latency'
        && node.number === previous.number + 1) {
        const start = previous.start || previous.number;
        previous.kind = 'latency-group';
        previous.start = start;
        previous.number = node.number;
        previous.label = `HOP ${String(start).padStart(2, '0')}-${String(node.number).padStart(2, '0')}`;
        previous.rtt = node.rtt;
        previous.delta += node.delta;
        previous.detail = `+${Math.round(previous.delta)} ms  MULTIPLE LATENCY JUMPS`;
        return;
      }
      merged.push({...node});
    });
    return merged;
  }

  function summarizeRoute(route) {
    const hops = Array.isArray(route.hops) ? route.hops : [];
    const maxNodes = Number.isInteger(route.maxNodes) ? Math.max(3, route.maxNodes) : 6;
    if (hops.length === 0) return [];

    const last = hops[hops.length - 1];
    const targetReached = route.status === 'complete' && last.target;
    const middle = hops.slice(0, targetReached ? -1 : undefined);
    const selected = selectImportant(middle, maxNodes);
    const nodes = [{kind: 'local', label: 'LOCAL', rtt: '', detail: ''}];
    let group = [];
    function flushGroup() {
      if (group.length) nodes.push(groupedNode(group));
      group = [];
    }
    middle.forEach((hop) => {
      if (!selected.has(hop.number)) {
        group.push(hop);
        return;
      }
      flushGroup();
      nodes.push(exactNode(hop));
    });
    flushGroup();
    nodes.push(targetReached
      ? exactNode(last, 'target')
      : {kind: 'incomplete', label: 'TARGET', rtt: '--', detail: 'ROUTE INCOMPLETE'});
    return mergeConsecutiveLatency(nodes);
  }

  function nodeHTML(node) {
    const detail = node.detail ? `<span class="node-status">${node.detail}</span>` : '';
    return `<li class="route-node is-${node.kind}">
      <span class="node-marker" aria-hidden="true"></span>
      <span class="node-label">${node.label}</span>
      <strong class="node-rtt">${node.rtt}</strong>
      ${detail}
    </li>`;
  }

  namespace.createRouteView = function createRouteView(root = document) {
    const element = root.querySelector('#route-nodes');
    let lastRouteJSON = '';
    return function renderRoute(snapshot) {
      const route = snapshot.route || {status: 'measuring', hops: []};
      const routeJSON = JSON.stringify(route);
      if (routeJSON === lastRouteJSON) return;
      lastRouteJSON = routeJSON;
      const nodes = summarizeRoute(route);
      if (nodes.length) {
        element.innerHTML = nodes.map(nodeHTML).join('');
        return;
      }
      const status = route.status === 'unavailable' ? 'UNAVAILABLE' : 'MEASURING';
      element.innerHTML = nodeHTML({kind: 'muted', label: 'ROUTE', rtt: '', detail: status});
    };
  };

  namespace.route = {summarizeRoute};
}(window.NetworkMonitor = window.NetworkMonitor || {}));
