(() => {
  'use strict';

  const state = { event: null, identity: '', selected: '', selectedLink: -1, loadingView: null, root: null, svg: null, list: null, stage: null, inspector: null, inspectorCollapsed: true, fullscreen: false, fullscreenFocus: null, savedBodyOverflow: null, zoomOutput: null, scale: 1, x: 0, y: 0, base: null, pointers: new Map(), dragging: false, moved: false, filters: {}, zoomFrame: 0, nodes: [], nodeIndex: new Map(), projectedIndex: new Map(), snapshotIndex: new Map(), renderedNodes: new Map() };
  const $ = (id) => document.getElementById(id);
  const node = (tag, className, text) => { const item = document.createElement(tag); if (className) item.className = className; if (text !== undefined) item.textContent = String(text); return item; };
  const safeText = (value, fallback = 'Unavailable') => value === undefined || value === null || value === '' ? fallback : String(value);
  const modelValue = (value) => { if (value === undefined || value === null || value === '') return 'unavailable'; let result; try { result = typeof value === 'object' ? JSON.stringify(value) : String(value); } catch (_) { result = String(value); } return result.length > 500 ? `${result.slice(0, 497)}…` : result; };

  function setup() {
    if (state.root) return;
    const visual = document.getElementById('exploration-visual') || document.querySelector('.exploration-visual');
    if (!visual) return;
    state.root = visual.querySelector('.exploration-topology-wrap');
    state.svg = $('exploration-topology'); state.list = $('exploration-topology-list');
    if (!state.root || !state.svg || !state.list) return;
    state.heading = state.root.querySelector('h3'); state.legend = state.root.querySelector('.exploration-legend');
    state.root.classList.add('topology-workbench');
    const toolbar = node('div', 'topology-toolbar'); toolbar.setAttribute('role', 'toolbar'); toolbar.setAttribute('aria-label', 'Topology controls');
    const searchLabel = node('label', 'topology-search-label', 'Find a device'); searchLabel.htmlFor = 'topology-search';
    const search = node('input', 'topology-search'); search.id = 'topology-search'; search.type = 'search'; search.placeholder = 'Search devices'; search.autocomplete = 'off'; search.setAttribute('aria-controls', 'topology-device-list');
    const searchWrap = node('div', 'topology-search-wrap'); searchWrap.append(searchLabel, search);
    const toolButton = (id, text, label) => { const button = node('button', 'topology-tool-button', text); button.id = id; button.type = 'button'; button.setAttribute('aria-label', label); return button; };
    const fit = toolButton('topology-fit', 'Fit', 'Fit topology to view'); const zoomOut = toolButton('topology-zoom-out', '−', 'Zoom out'); const zoom = node('output', 'topology-zoom', '100%'); zoom.id = 'topology-zoom'; zoom.setAttribute('aria-live', 'polite'); const zoomIn = toolButton('topology-zoom-in', '+', 'Zoom in'); const focus = toolButton('topology-focus', 'Focus selected', 'Focus the selected device');
    const layer = (id, label, checked) => { const wrap = node('label', 'topology-layer'); const input = node('input'); input.type = 'checkbox'; input.id = id; input.checked = checked; wrap.append(input, document.createTextNode(label)); return { wrap, input }; };
    const forward = layer('topology-show-forward', 'Forward', true); const returned = layer('topology-show-return', 'Return', true);
    const inspectorToggle = toolButton('topology-inspector-toggle', 'Details', 'Show selected device details'); inspectorToggle.setAttribute('aria-controls','topology-inspector'); inspectorToggle.setAttribute('aria-expanded','false');
    const fullscreenToggle = toolButton('topology-fullscreen','Full screen','Expand topology canvas'); fullscreenToggle.setAttribute('aria-pressed','false');
    const controls = node('div', 'topology-toolbar-actions'); controls.append(fit, zoomOut, zoom, zoomIn, focus, inspectorToggle, fullscreenToggle, forward.wrap, returned.wrap);
    const filters = node('div', 'topology-filter-bar'); filters.id = 'topology-filters'; filters.setAttribute('aria-label', 'Topology filters');
    const resultControls = node('div','topology-result-controls'); resultControls.id = 'topology-result-controls'; resultControls.hidden = true;
    const resultSelector = (id,label,sourceId) => { const wrap = node('label','topology-result-select-label',label); const select = node('select','topology-result-select'); select.id = id; select.setAttribute('aria-label',label); wrap.append(select); select.addEventListener('change',() => { const source = $(sourceId); if (!source || source.value === select.value) return; source.value = select.value; source.dispatchEvent(new Event('change',{bubbles:true})); }); return {wrap,select}; };
    const branchControl = resultSelector('topology-branch-select','Forward branch','exploration-branch'); const returnControl = resultSelector('topology-return-select','Return candidate','exploration-return'); const resultStatus = node('span','topology-result-status'); resultStatus.id = 'topology-result-status'; resultStatus.setAttribute('aria-live','polite'); resultControls.append(branchControl.wrap,returnControl.wrap,resultStatus);
    const traceBadge = node('div','topology-trace-badge'); traceBadge.id = 'topology-trace-badge'; traceBadge.hidden = true; traceBadge.setAttribute('role','status');
    toolbar.append(searchWrap, controls);
    const deviceList = node('div', 'topology-device-list'); deviceList.id = 'topology-device-list'; deviceList.setAttribute('role', 'group'); deviceList.setAttribute('aria-label', 'Device search results');
    const body = node('div', 'topology-workbench-body topology-inspector-collapsed'); const stage = node('div', 'topology-stage'); stage.id = 'topology-stage'; stage.tabIndex = 0; stage.setAttribute('role', 'region'); stage.setAttribute('aria-label', 'Interactive network topology. Use the device list to select a device.');
    const caption = node('div', 'topology-stage-caption', 'Declared topology draft · analysis not run · runtime observations unavailable'); caption.id = 'topology-caption'; caption.setAttribute('aria-live', 'polite');
    const canvas = node('div', 'topology-canvas'); canvas.append(state.svg);
    const minimap = node('button', 'topology-minimap'); minimap.type = 'button'; minimap.id = 'topology-minimap'; minimap.setAttribute('aria-label', 'Center topology overview'); minimap.title = 'Click to center the canvas. Use arrow keys to pan, + / − to zoom, and 0 to fit.';
    const miniSvg = svgElement('svg', { id: 'topology-minimap-svg', 'aria-hidden': 'true', preserveAspectRatio: 'xMidYMid meet' }); minimap.append(node('span', 'topology-minimap-label', 'OVERVIEW'), miniSvg);
    stage.append(canvas, minimap, caption); stage.setAttribute('aria-keyshortcuts', 'ArrowUp ArrowDown ArrowLeft ArrowRight + - 0');
    const overview = node('div', 'topology-overview-bar'); overview.id = 'topology-overview'; overview.setAttribute('aria-label', 'Topology overview');
    const inspector = node('aside', 'topology-inspector'); inspector.id = 'topology-inspector'; inspector.setAttribute('aria-label', 'Selected device details'); inspector.append(node('div', 'topology-inspector-empty', 'Select a device to inspect its saved configuration and selected path context.'));
    body.append(stage, inspector);
    const help = node('p', 'topology-workbench-note', 'Declared links come from the supplied topology. Forward and return overlays show configured route analysis; live session observations are unavailable.');
    const trafficStatus = node('span', 'topology-traffic-status', 'Traffic observations unavailable'); trafficStatus.dataset.available = 'false'; trafficStatus.setAttribute('aria-live', 'polite'); help.append(document.createTextNode(' '), trafficStatus);
    state.list.replaceChildren();
    state.root.append(overview, toolbar, resultControls, traceBadge, filters, deviceList, body, help);
    state.overview = overview; state.minimap = minimap; state.miniSvg = miniSvg;
    state.deviceList = deviceList; state.stage = stage; state.inspector = inspector; state.inspectorToggle = inspectorToggle; state.fullscreenToggle = fullscreenToggle; state.body = body; state.zoomOutput = zoom; state.search = search; state.caption = caption; state.forwardToggle = forward.input; state.returnToggle = returned.input; state.toolbar = toolbar; state.filtersHost = filters; state.resultControls = resultControls; state.branchSelect = branchControl.select; state.returnSelect = returnControl.select; state.resultStatus = resultStatus; state.traceBadge = traceBadge;
    inspectorToggle.addEventListener('click', () => setInspectorCollapsed(!state.inspectorCollapsed));
    fullscreenToggle.addEventListener('click', () => setFullscreen(!state.fullscreen));
    const applyZoom = () => { if (state.zoomFrame) return; state.zoomFrame = requestAnimationFrame(() => { state.zoomFrame = 0; if (!state.base) return; const width = state.base.width / state.scale; const height = state.base.height / state.scale; state.svg.setAttribute('viewBox', `${state.x} ${state.y} ${width} ${height}`); state.svg.classList.toggle('topology-compact-labels', state.scale < .65); state.zoomOutput.value = `${Math.round(state.scale * 100)}%`; state.zoomOutput.textContent = `${Math.round(state.scale * 100)}%`; if (state.miniViewport) { state.miniViewport.setAttribute('x',state.x); state.miniViewport.setAttribute('y',state.y); state.miniViewport.setAttribute('width',width); state.miniViewport.setAttribute('height',height); } }); };
    state.applyZoom = applyZoom;
    const svgPoint = (clientX, clientY) => { const matrix = state.svg.getScreenCTM(); if (!matrix) return null; const point = state.svg.createSVGPoint(); point.x = clientX; point.y = clientY; const rendered = point.matrixTransform(matrix.inverse()); const box = state.svg.viewBox.baseVal; if (!state.base || !box.width || !box.height) return rendered; return { x: state.x + (rendered.x - box.x) * (state.base.width / state.scale) / box.width, y: state.y + (rendered.y - box.y) * (state.base.height / state.scale) / box.height }; };
    const setScale = (value, clientX, clientY) => { const next = Math.max(.25, Math.min(4, value)); const anchor = clientX === undefined ? null : svgPoint(clientX, clientY); if (anchor && state.base) { const px = (anchor.x - state.x) / (state.base.width / state.scale); const py = (anchor.y - state.y) / (state.base.height / state.scale); state.scale = next; state.x = anchor.x - px * state.base.width / next; state.y = anchor.y - py * state.base.height / next; } else state.scale = next; applyZoom(); };
    state.fit = () => { if (!state.base) return; state.scale = 1; state.x = state.base.x; state.y = state.base.y; applyZoom(); };
    fit.addEventListener('click', () => state.fit()); zoomIn.addEventListener('click', () => setScale(state.scale * 1.2)); zoomOut.addEventListener('click', () => setScale(state.scale / 1.2));
    focus.addEventListener('click', () => { if (state.selected) selectNode(state.selected, true, true); else state.search.focus(); });
    search.addEventListener('input', () => { renderDeviceList(); const reset = $('topology-filter-reset'); if (reset) reset.disabled = !Object.values(state.filters).some(Boolean) && !search.value; });
    forward.input.addEventListener('change', () => { state.svg.classList.toggle('topology-hide-forward', !forward.input.checked); });
    returned.input.addEventListener('change', () => { state.svg.classList.toggle('topology-hide-return', !returned.input.checked); });
    stage.addEventListener('wheel', (event) => { event.preventDefault(); setScale(state.scale * (event.deltaY < 0 ? 1.12 : 1 / 1.12), event.clientX, event.clientY); }, { passive: false });
    stage.addEventListener('keydown', (event) => { if (event.target.closest('.topology-node,.topology-link') || event.altKey || event.ctrlKey || event.metaKey || !state.base) return; const step = state.base.width / state.scale * .08; if (event.key === 'ArrowLeft') state.x -= step; else if (event.key === 'ArrowRight') state.x += step; else if (event.key === 'ArrowUp') state.y -= step; else if (event.key === 'ArrowDown') state.y += step; else if (event.key === '+' || event.key === '=') setScale(state.scale * 1.2); else if (event.key === '-') setScale(state.scale / 1.2); else if (event.key === '0' || event.key === 'Home') state.fit(); else return; event.preventDefault(); applyZoom(); });
    minimap.addEventListener('click', (event) => { if (!state.base) return; if (event.detail === 0) { state.fit(); return; } const matrix = miniSvg.getScreenCTM(); if (!matrix) return; const point = miniSvg.createSVGPoint(); point.x = event.clientX; point.y = event.clientY; const target = point.matrixTransform(matrix.inverse()); state.x = target.x - state.base.width / state.scale / 2; state.y = target.y - state.base.height / state.scale / 2; applyZoom(); });
    stage.addEventListener('pointerdown', (event) => { if (event.button !== 0 || event.target.closest('.topology-node,.topology-link,.topology-minimap')) return; state.dragging = true; state.moved = false; state.pointers.set(event.pointerId, { x: event.clientX, y: event.clientY }); stage.setPointerCapture(event.pointerId); stage.classList.add('is-panning'); });
    stage.addEventListener('pointermove', (event) => { if (!state.dragging || !state.pointers.has(event.pointerId)) return; const previous = state.pointers.get(event.pointerId); const oldPoint = svgPoint(previous.x, previous.y); const nextPoint = svgPoint(event.clientX, event.clientY); state.pointers.set(event.pointerId, { x: event.clientX, y: event.clientY }); if (Math.abs(event.clientX - previous.x) + Math.abs(event.clientY - previous.y) > 1) state.moved = true; if (state.base && oldPoint && nextPoint) { state.x -= nextPoint.x - oldPoint.x; state.y -= nextPoint.y - oldPoint.y; applyZoom(); } });
    const endPan = (event) => { state.pointers.delete(event.pointerId); if (!state.pointers.size) { state.dragging = false; stage.classList.remove('is-panning'); setTimeout(() => { state.moved = false; }, 0); } };
    stage.addEventListener('pointerup', endPan); stage.addEventListener('pointercancel', endPan);
    state.svg.addEventListener('click', (event) => { if (state.moved) { event.preventDefault(); event.stopImmediatePropagation(); return; } const link = event.target.closest('.topology-link'); if (link) { event.preventDefault(); event.stopImmediatePropagation(); selectLink(Number(link.dataset.linkIndex)); return; } const target = event.target.closest('.topology-node'); if (!target) return; event.preventDefault(); event.stopImmediatePropagation(); selectNode(target.dataset.nodeId, false, true); }, true);
    state.svg.addEventListener('keydown', (event) => { if (!['Enter', ' '].includes(event.key)) return; const link = event.target.closest('.topology-link'); if (link) { event.preventDefault(); event.stopImmediatePropagation(); selectLink(Number(link.dataset.linkIndex)); return; } const target = event.target.closest('.topology-node'); if (!target) return; event.preventDefault(); event.stopImmediatePropagation(); selectNode(target.dataset.nodeId, false, true); }, true);
    state.filtersHost = filters; state.trafficStatus = trafficStatus;
    window.addEventListener('resize', () => { if (state.base) applyZoom(); });
    document.addEventListener('keydown', (event) => { const sourceLines = $('source-lines'); const sourceVisible = Boolean(sourceLines && !sourceLines.hidden && sourceLines.getClientRects().length && getComputedStyle(sourceLines).visibility !== 'hidden' && getComputedStyle(sourceLines).display !== 'none'); if (event.key === 'Escape' && state.fullscreen && !sourceVisible) { event.preventDefault(); setFullscreen(false); } }, true);
    state.svg.classList.toggle('topology-hide-forward', !forward.input.checked); state.svg.classList.toggle('topology-hide-return', !returned.input.checked);
  }

  function getRequest() { const e = state.event || {}; return e.request || e.exploration && e.exploration.request || {}; }
  function indexPresentation() { const requested = getRequest().nodes || []; const requestedIndex = new Map(requested.map((item) => [item.node_id,item])); const projected = getTopology().nodes || []; state.projectedIndex = new Map(projected.map((item) => [item.node_id || item.id,item])); state.snapshotIndex = new Map((state.event.snapshots || []).filter(Boolean).map((item) => [item.node_id,item.snapshot])); state.nodes = projected.length ? projected.map((item) => ({ ...(requestedIndex.get(item.node_id || item.id) || {}), ...item, node_id: item.node_id || item.id })) : requested; state.nodeIndex = new Map(state.nodes.map((item) => [item.node_id,item])); }
  function getNodes() { return state.nodes; }
  function roleAnnotationKey(nodeId) { const item = state.nodeIndex.get(nodeId); return `${nodeId}\u0000${item && item.snapshot_id || ''}`; }
  function getBranch() { const e = state.event || {}; const branches = e.exploration && e.exploration.result && e.exploration.result.branches || []; return branches[e.branchIndex] || null; }
  function getReturn() { if (state.event && state.event.mode === 'trace') return getTraceResult() && getTraceResult().return || null; const branch = getBranch(); return branch && branch.returns && branch.returns[state.event.returnIndex] || null; }
  function getTraceResult() { return state.event && state.event.investigation && state.event.investigation.result || null; }
  function getHops(direction) { if (state.event && state.event.mode === 'trace') { const side = getTraceResult() && getTraceResult()[direction]; return side && side.hops || []; } if (direction === 'forward') return getBranch() && getBranch().forward && getBranch().forward.hops || []; return getReturn() && getReturn().direction && getReturn().direction.hops || []; }
  function getSnapshot(id) { return state.snapshotIndex.get(id) || null; }
  function getTopology() { const e = state.event || {}; return e.topology || e.projection || {}; }
  function renderResultControls(detail) {
    const sourceBranch = $('exploration-branch'), sourceReturn = $('exploration-return');
    const mirror = (target, source) => {
      if (!source || !source.options || !source.options.length) { target.replaceChildren(); target.parentElement.hidden = true; return false; }
      const selected = source.value;
      target.replaceChildren(...Array.from(source.options, (option) => option.cloneNode(true)));
      target.value = selected;
      target.parentElement.hidden = false;
      return true;
    };
    const resultMode = detail && detail.mode === 'result';
    const hasBranches = resultMode && mirror(state.branchSelect, sourceBranch);
    const hasReturns = resultMode && mirror(state.returnSelect, sourceReturn);
    state.resultControls.hidden = !(hasBranches || hasReturns);
    state.traceBadge.hidden = !(detail && detail.mode === 'trace');
    if (state.traceBadge.hidden) state.traceBadge.textContent = '';
    else {
      const trace = detail.investigation && detail.investigation.result || detail.trace || {};
      const side = (name) => { const value = trace[name] || {}; return `${name === 'forward' ? 'Forward' : 'Return'} ${value.disposition || 'status unavailable'}`; };
      const parts = [side('forward'), side('return')];
      if (trace.session_status) parts.push(`Session ${trace.session_status}`);
      if (trace.asymmetric === true || trace.return && trace.return.asymmetric === true) parts.push('Asymmetric');
      state.traceBadge.textContent = parts.join(' · ');
    }
    if (!state.resultControls.hidden) {
      const status = $('exploration-return-status');
      const branch = getBranch(), candidate = getReturn();
      const derived = candidate ? `Return ${candidate.disposition || candidate.direction && candidate.direction.disposition || 'status unavailable'}${candidate.asymmetric ? ' · asymmetric' : ''}${candidate.session_status ? ` · session ${candidate.session_status}` : ''}` : branch ? 'Return candidate unavailable' : '';
      state.resultStatus.textContent = status && status.textContent.trim() ? status.textContent.trim() : derived;
    } else state.resultStatus.textContent = '';
  }
  function getProjectedNode(requestNode) { return state.projectedIndex.get(requestNode.node_id) || {}; }
  function projectedValue(item, key) { const projected = getProjectedNode(item); const snapshot = getSnapshot(item.node_id) || {}; const presentation = projected.presentation || {}; const inventory = projected.inventory || {}; return projected[key] ?? presentation[key] ?? inventory[key] ?? snapshot[key] ?? null; }
  function projectedFacetValues(item, key) {
    const projected = getProjectedNode(item); const values = [];
    const add = (value) => { (Array.isArray(value) ? value : value === undefined || value === null || value === '' ? [] : [value]).forEach((entry) => { const text = String(entry); if (text && !values.includes(text)) values.push(text); }); };
    add(projectedValue(item,key)); (projected.interfaces || []).forEach((iface) => add(iface && iface[key]));
    return values;
  }
  const svgElement = (tag, attrs = {}, parent) => { const element = document.createElementNS('http://www.w3.org/2000/svg', tag); Object.entries(attrs).forEach(([key, value]) => { if (value !== undefined && value !== null) element.setAttribute(key, String(value)); }); if (parent) parent.append(element); return element; };
  function fittedSvgLabel(element, value, maxWidth, minimumSize) {
    const full = String(value ?? ''); element.textContent = full; element.setAttribute('aria-label',full);
    const title = svgElement('title',{},element); title.textContent = full;
    let size = Number(element.getAttribute('font-size')) || 16; const minimum = minimumSize || 11;
    const measured = () => element.getComputedTextLength ? element.getComputedTextLength() : String(element.firstChild && element.firstChild.textContent || '').length * size * .58;
    while (size > minimum && measured() > maxWidth) { size -= 1; element.setAttribute('font-size',size); element.style.fontSize = `${size}px`; }
    if (measured() > maxWidth) {
      let shown = full;
      while (shown.length > 1) { shown = `${shown.slice(0,-2)}…`; element.firstChild.textContent = shown; if (measured() <= maxWidth) break; }
    }
  }
  function deviceIcon(kind, parent) {
    const icon = svgElement('g', { class: 'topology-device-icon', 'data-icon': kind, 'aria-hidden': 'true', transform: 'translate(0 0)' }, parent);
    const paths = {
      firewall: 'M2 5h28v20H2z M2 12h28 M2 19h28 M11 5v7 M21 12v7 M11 19v6 M21 5v7',
      router: 'M4 15 16 5l12 10-12 10z M16 5v20 M4 15h24 M8 11l-4 4 4 4 M24 11l4 4-4 4',
      switch: 'M3 8h26v16H3z M7 13h3m3 0h3m3 0h3m3 0h3 M8 19h2m4 0h2m4 0h2m4 0h1',
      host: 'M5 3h22v25H5z M10 8h12v8H10z M10 21h3m4 0h3',
      server: 'M5 3h22v9H5z M5 17h22v9H5z M9 7h2m3 0h2m-7 14h2m3 0h2',
      cloud: 'M8 24a6 6 0 0 1-.5-12A9 9 0 0 1 25 10a7 7 0 0 1 1 14z',
      network: 'M16 5 7 23h18z M16 5v12 M7 23l9-6 9 6 M13 5a3 3 0 1 0 6 0 3 3 0 1 0-6 0 M4 23a3 3 0 1 0 6 0 3 3 0 1 0-6 0 M22 23a3 3 0 1 0 6 0 3 3 0 1 0-6 0',
      device: 'M3 5h26v22H3z M7 11h18 M7 17h4m4 0h4'
    };
    svgElement('path', { d: paths[kind] || paths.device }, icon); return icon;
  }
  function normalizedRole(value) { const text = String(value || '').toLowerCase(); return ['firewall','router','switch','host','server','cloud','network'].includes(text) ? text : 'device'; }
  function renderGraph() {
    const d3 = window.d3;
    if (!d3 || typeof d3.forceSimulation !== 'function' || typeof d3.forceX !== 'function' || typeof d3.forceY !== 'function' || typeof d3.forceCollide !== 'function') { showTopologyStatus('error','The topology layout engine did not load. Reload this page to restore the graph view.'); return; }
    const request = getRequest(); const nodes = getNodes(); const topology = getTopology();
    const projectedLinks = Array.isArray(topology.links) ? topology.links : request.links || [];
    const links = projectedLinks.filter((item) => item && item.a && item.b && item.a.node_id && item.b.node_id);
    const layout = nodes.map((item, index) => { const point = getProjectedNode(item).layout || {}; return { item, column: Number.isFinite(Number(point.column)) ? Number(point.column) : 0, row: Number.isFinite(Number(point.row)) ? Number(point.row) : index }; });
    const degrees = new Map(); links.forEach((link) => [link.a.node_id,link.b.node_id].forEach((id) => degrees.set(id,(degrees.get(id) || 0) + 1)));
    const maxDegree = Math.max(1,...degrees.values()); const cardW = 172; const cardH = Math.max(96,30 + maxDegree * 20); const gapY = 50, marginX = 36, marginY = 34;
    const maxColumns = Math.max(1, ...layout.map((entry) => entry.column + 1)); const maxRows = Math.max(1, ...layout.map((entry) => entry.row + 1));
    const availableWidth = Math.max(480, (state.stage && state.stage.clientWidth || 960) - 24); const desiredPitch = maxColumns > 1 ? (availableWidth - marginX * 2 - cardW) / (maxColumns - 1) : cardW + 120; const gapX = Math.max(128, Math.min(176, desiredPitch - cardW));
    const width = Math.max(480, marginX * 2 + cardW + (maxColumns - 1) * (cardW + gapX) + 48);
    const height = Math.max(280, marginY * 2 + cardH + (maxRows - 1) * (cardH + gapY));
    const geometry = layout.map(({ item, column, row }) => { const anchorX = marginX + column * (cardW + gapX) + cardW / 2; const anchorY = marginY + row * (cardH + gapY) + cardH / 2; return { nodeId: item.node_id, anchorX, anchorY, x: anchorX, y: anchorY }; });
    if (geometry.length) {
      const simulation = d3.forceSimulation(geometry).force('x', d3.forceX((entry) => entry.anchorX).strength(.92)).force('y', d3.forceY((entry) => entry.anchorY).strength(.92)).force('collision', d3.forceCollide(cardH / 2 + 14).strength(1).iterations(3)).stop();
      for (let tick = 0; tick < 90; tick += 1) simulation.tick();
    }
    const positions = new Map(geometry.map((entry) => [entry.nodeId, { x: entry.x - cardW / 2, y: entry.y - cardH / 2 }]));
    state.svg.replaceChildren(); state.svg.setAttribute('viewBox', `0 0 ${width} ${height}`); state.svg.setAttribute('preserveAspectRatio', 'xMidYMid meet');
    state.base = { x: 0, y: 0, width, height }; state.x = 0; state.y = 0; state.scale = 1;
    const defs = svgElement('defs', {}, state.svg);
    [['forward','#27825f'],['return','#557eae'],['blocked','#bf5158'],['unknown','#c3913d']].forEach(([id,color]) => { const marker = svgElement('marker', { id: `topology-arrow-${id}`, viewBox: '0 0 10 10', refX: '8', refY: '5', markerWidth: '5', markerHeight: '5', markerUnits: 'userSpaceOnUse', orient: 'auto' }, defs); svgElement('path', { d: 'M0 0L10 5L0 10z', fill: color }, marker); });
    const linkLayer = svgElement('g', { class: 'topology-links' }, state.svg); const overlayLayer = svgElement('g', { class: 'topology-overlays' }, state.svg); const nodeLayer = svgElement('g', { class: 'topology-nodes' }, state.svg);
    const linkIndex = new Map(); const endpointLinks = new Map();
    links.forEach((link) => [link.a.node_id,link.b.node_id].forEach((id) => { if (!endpointLinks.has(id)) endpointLinks.set(id,[]); endpointLinks.get(id).push(link); }));
    const endpointOffset = (id, link) => { const list = endpointLinks.get(id) || []; if (list.length < 2) return 0; const slot = list.indexOf(link); return -cardH / 2 + 16 + slot * (cardH - 32) / (list.length - 1); };
    links.forEach((link) => {
      const index = projectedLinks.indexOf(link);
      const a = positions.get(link.a.node_id), b = positions.get(link.b.node_id); if (!a || !b) return;
      const sameColumn = Math.abs(b.x - a.x) < cardW; const direction = b.x >= a.x ? 1 : -1; const sx = a.x + (sameColumn || direction > 0 ? cardW : 0); const ex = b.x + (sameColumn || direction < 0 ? cardW : 0); const sy = a.y + cardH / 2 + endpointOffset(link.a.node_id,link); const ey = b.y + cardH / 2 + endpointOffset(link.b.node_id,link); const bend = Math.max(48, Math.abs(ex - sx) * .48);
      const laneX = Math.max(sx,ex) + 86 + (index % 3) * 12;
      const path = sameColumn ? `M${sx} ${sy} C${laneX} ${sy},${laneX} ${ey},${ex} ${ey}` : `M${sx} ${sy} C${sx + direction * bend} ${sy},${ex - direction * bend} ${ey},${ex} ${ey}`;
      const reversePath = sameColumn ? `M${ex} ${ey} C${laneX} ${ey},${laneX} ${sy},${sx} ${sy}` : `M${ex} ${ey} C${ex - direction * bend} ${ey},${sx + direction * bend} ${sy},${sx} ${sy}`;
      const group = svgElement('g', { class: `topology-link${index === state.selectedLink ? ' topology-link-selected' : ''}`, 'data-link-index': index, 'data-provenance': link.provenance || 'declared_topology', 'data-synthetic': String(Boolean(link.synthetic)), tabindex: '0', role: 'button', 'aria-pressed': String(index === state.selectedLink), 'aria-label': `Link ${link.a.node_id} ${link.a.interface || 'port unavailable'} to ${link.b.node_id} ${link.b.interface || 'port unavailable'}` }, linkLayer); linkIndex.set(link, { path, reversePath, group, index });
      svgElement('path', { d: path, class: 'topology-link-line' }, group);
      const title = svgElement('title', {}, group); title.textContent = `${link.a.node_id} · ${link.a.interface || 'port unavailable'} ↔ ${link.b.node_id} · ${link.b.interface || 'port unavailable'}`;
      const portLabel = (x, y, label, side, lane) => { const text = label || 'port unavailable'; const width = Math.min(112, Math.max(48, text.length * 6.1 + 14)); const lx = side < 0 ? x - width - 7 : x + 7; const ly = y - 11 + lane * 14; const pill = svgElement('g', { class: 'topology-port-label', 'aria-label': text }, group); const title = svgElement('title',{},pill); title.textContent = text; svgElement('rect', { x: lx, y: ly, width, height: 22, rx: 7 }, pill); const t = svgElement('text', { x: lx + width / 2, y: ly + 15 }, pill); t.textContent = text.length > 17 ? `${text.slice(0,14)}…` : text; };
      portLabel(sx, sy, link.a.interface, sameColumn || direction > 0 ? 1 : -1, -1); portLabel(ex, ey, link.b.interface, sameColumn || direction < 0 ? 1 : -1, 1);
    });
    function overlay(hops, direction, status) {
      (hops || []).forEach((hop) => { if (!hop.link || !hop.link.a || !hop.link.b) return; const key = (edge) => [edge.a,edge.b].map((endpoint) => `${endpoint.node_id}\u0000${String(endpoint.interface || '').toLowerCase()}`).sort().join('\u0001'); const hopKey = key(hop.link); const match = links.find((link) => key(link) === hopKey); const metadata = match && linkIndex.get(match); if (!metadata) return; const value = String(status || hop.path && hop.path.disposition || 'unknown').toLowerCase(); const marker = value === 'blocked' ? 'blocked' : ['unknown','indeterminate','unresolved','not_evaluated'].includes(value) ? 'unknown' : direction; const d = hop.node_id === match.b.node_id ? metadata.reversePath : metadata.path; svgElement('path', { d, class: `topology-edge topology-${direction} topology-state-${value.replace(/[^a-z0-9-]/g, '-')}`, 'marker-end': `url(#topology-arrow-${marker})` }, overlayLayer); });
    }
    const branch = getBranch(), returnCandidate = getReturn();
    const forward = state.event && state.event.mode === 'trace' ? getHops('forward') : branch && branch.forward && branch.forward.hops || [];
    const returned = state.event && state.event.mode === 'trace' ? getHops('return') : returnCandidate && returnCandidate.direction && returnCandidate.direction.hops || [];
    overlay(forward, 'forward', state.event && state.event.mode === 'trace' ? getTraceResult()?.forward?.disposition : branch && branch.forward && branch.forward.disposition);
    overlay(returned, 'return', state.event && state.event.mode === 'trace' ? getTraceResult()?.return?.disposition : returnCandidate && returnCandidate.disposition);
    const hits = new Map(); [[forward,'forward'],[returned,'return']].forEach(([hops,direction]) => (hops || []).forEach((hop,index) => { if (!hits.has(hop.node_id)) hits.set(hop.node_id,[]); hits.get(hop.node_id).push({hop,index,direction}); }));
    nodes.forEach((item) => {
      const pos = positions.get(item.node_id); if (!pos) return; const projected = getProjectedNode(item); const annotationKey = roleAnnotationKey(item.node_id); const override = state.roleAnnotations && state.roleAnnotations[annotationKey]; const role = normalizedRole(override || projected.icon || projected.role || projected.device_role); const icon = override ? role : normalizedRole(projected.icon || role); const roleSource = override ? 'display_annotation' : projected.role_source || 'unspecified'; const found = hits.get(item.node_id) || []; const directions = [...new Set(found.map((entry) => entry.direction))];
      const facetData = Object.fromEntries(['zone','subnet','vrf','vdom'].map((field) => [field,projectedFacetValues(item,field).join(',')]));
      const snapshot = getSnapshot(item.node_id); const hostname = projected.hostname || snapshot && snapshot.hostname; const primaryLabel = String(projected.label || item.node_id); const accessibleLabel = `${primaryLabel}${hostname && hostname !== primaryLabel ? `, ${hostname}` : ''}, ${role === 'device' ? 'device type unspecified' : role}${roleSource === 'display_annotation' ? ', display annotation' : ''}${directions.length ? `, selected ${directions.join(' and ')} path` : ', no selected path overlay'}`;
      const group = svgElement('g', { class: `topology-node topology-${role} topology-${icon} ${directions.map((value) => `topology-${value}`).join(' ')}`, 'data-node-id': item.node_id, 'data-role': role, 'data-icon': icon, 'data-role-source': roleSource, 'data-zone': facetData.zone, 'data-subnet': facetData.subnet, 'data-vrf': facetData.vrf, 'data-vdom': facetData.vdom, tabindex: '0', role: 'button', 'aria-pressed': String(state.selected === item.node_id), 'aria-label': accessibleLabel }, nodeLayer);
      const nodeTitle = svgElement('title',{},group); nodeTitle.textContent = accessibleLabel;
      const fill = role === 'firewall' ? '#fff7eb' : role === 'router' ? '#eef6ff' : role === 'switch' ? '#eef9f5' : role === 'server' ? '#f4f1ff' : role === 'host' ? '#f4f7fa' : '#f2f6f8';
      svgElement('rect', { class: 'topology-node-card', x: pos.x, y: pos.y, width: cardW, height: cardH, rx: 14, fill }, group);
      const iconWrap = svgElement('g', { transform: `translate(${pos.x + 14} ${pos.y + 12}) scale(1.28)` }, group); deviceIcon(icon, iconWrap);
      const label = svgElement('text', { class: 'topology-node-label', x: pos.x + 56, y: pos.y + 34, 'font-size': '16' }, group); fittedSvgLabel(label,primaryLabel,104,13);
      const sub = svgElement('text', { class: 'topology-node-subtitle', x: pos.x + 56, y: pos.y + 57, 'font-size': '12' }, group); const secondary = hostname && hostname !== primaryLabel ? hostname : primaryLabel !== item.node_id ? item.node_id : role === 'device' ? 'Device type unspecified' : role; fittedSvgLabel(sub,secondary,104,10);
      const status = svgElement('text', { class: 'topology-node-status', x: pos.x + 16, y: pos.y + 84, 'font-size': '11' }, group); fittedSvgLabel(status,found.length ? `${directions.join(' + ')} path · ${found[0].hop.path && found[0].hop.path.disposition || 'status unavailable'}` : roleSource === 'query_endpoint' ? 'Query endpoint' : 'Operator supplied device',140,9);
      if (topology.traffic && topology.traffic.available === true) { const trafficValue = projected.traffic || topology.traffic.by_node && topology.traffic.by_node[item.node_id]; if (trafficValue) { const traffic = svgElement('text', { class: 'topology-node-traffic', x: pos.x + cardW - 12, y: pos.y + 82, 'text-anchor': 'end' }, group); traffic.textContent = String(trafficValue.label || trafficValue.value || ''); } }
    });
    state.renderedNodes = new Map(Array.from(state.svg.querySelectorAll('.topology-node'), (item) => [item.dataset.nodeId,item]));
    renderMinimap(positions, links, cardW, cardH); renderFilters(); applyGraphFilters(); state.applyZoom(); return true;
  }
  function renderMinimap(positions, links, cardW, cardH) {
    if (!state.miniSvg || !state.base) return; state.miniSvg.replaceChildren(); state.miniSvg.setAttribute('viewBox', `0 0 ${state.base.width} ${state.base.height}`);
    links.forEach((link) => { const a = positions.get(link.a.node_id), b = positions.get(link.b.node_id); if (a && b) svgElement('line', { x1:a.x+cardW/2, y1:a.y+cardH/2, x2:b.x+cardW/2, y2:b.y+cardH/2, class:'topology-minimap-link' }, state.miniSvg); });
    positions.forEach((pos,id) => svgElement('rect', { x:pos.x, y:pos.y, width:cardW, height:cardH, rx:12, class:'topology-minimap-node', 'data-node-id':id }, state.miniSvg));
    state.miniViewport = svgElement('rect', { class:'topology-minimap-viewport', x:state.x, y:state.y, width:state.base.width/state.scale, height:state.base.height/state.scale }, state.miniSvg);
  }
  function renderFilters() {
    if (!state.filtersHost) return; const topology = getTopology(); const facets = topology.facets || {}; const fields = ['zone','subnet','vrf','vdom']; state.filtersHost.replaceChildren();
    fields.forEach((field) => { const facet = facets[field] || {}; const values = Array.isArray(facet) ? facet : facet.values || []; if (!values.length || facet.available === false) return; const label = node('label','topology-filter-label',field.toUpperCase()); const select = node('select','topology-filter'); select.dataset.filter = field; select.setAttribute('aria-label',`Filter by ${field}`); const all = node('option','',`All ${field}s`); all.value = ''; select.append(all); values.forEach((entry) => { const value = typeof entry === 'object' ? entry.value : entry; if (value === undefined || value === null || value === '') return; const option = node('option','',typeof entry === 'object' ? `${value} (${entry.count})` : value); option.value = String(value); select.append(option); }); select.value = state.filters[field] || ''; select.addEventListener('change', () => { state.filters[field] = select.value; applyGraphFilters(); renderDeviceList(); }); label.append(select); state.filtersHost.append(label); });
    const reset = node('button','topology-filter-reset','Reset filters'); reset.id = 'topology-filter-reset'; reset.type = 'button'; reset.addEventListener('click', () => { state.filters = {}; state.search.value = ''; renderFilters(); applyGraphFilters(); renderDeviceList(); }); state.filtersHost.append(reset);
  }
  function matchesFacetFilters(item) { return Object.entries(state.filters).every(([field,value]) => !value || projectedFacetValues(item,field).includes(value)); }
  function applyGraphFilters() {
    if (!state.svg) return; const nodes = getNodes(); const visible = new Set();
    nodes.forEach((item) => { const match = matchesFacetFilters(item); const group = state.renderedNodes.get(item.node_id); if (group) { group.classList.toggle('topology-filtered', !match); group.setAttribute('aria-hidden', String(!match)); group.setAttribute('tabindex', match ? '0' : '-1'); } if (match) visible.add(item.node_id); });
    state.svg.querySelectorAll('.topology-link').forEach((item) => { const index = Number(item.dataset.linkIndex); const link = (getTopology().links || getRequest().links || [])[index]; const filtered = Boolean(link && (!visible.has(link.a.node_id) || !visible.has(link.b.node_id))); item.classList.toggle('topology-filtered', filtered); item.setAttribute('aria-hidden',String(filtered)); item.setAttribute('tabindex',filtered ? '-1' : '0'); });
    const links = getTopology().links || getRequest().links || []; const facts = [['Devices',nodes.length],['Visible',visible.size],['Declared links',links.length],['View',state.event && state.event.mode === 'draft' ? 'Draft' : 'Configured paths']]; state.overview.replaceChildren(...facts.map(([label,value]) => { const stat = node('div','topology-overview-stat'); stat.append(node('span','',label),node('strong','',value)); return stat; })); const reset = $('topology-filter-reset'); if (reset) reset.disabled = !Object.values(state.filters).some(Boolean) && !state.search.value;
  }
  function deviceSearchText(item) { const snapshot = getSnapshot(item.node_id); const projected = getProjectedNode(item); return [item.node_id, projected.label, projected.role, projected.icon, projected.zone, projected.subnet, projected.vrf, projected.vdom, snapshot && snapshot.hostname, snapshot && snapshot.vendor, snapshot && snapshot.context].flat().filter(Boolean).join(' ').toLocaleLowerCase(); }
  function sourceButton(snapshotId, ref, label, host) { if (!snapshotId || !ref) return; const button = node('button', 'topology-source-button', label || `Source ${String(ref).slice(0, 18)}`); button.type = 'button'; button.dataset.snapshotId = snapshotId; button.dataset.evidenceRef = ref; button.addEventListener('click', () => document.dispatchEvent(new CustomEvent('graphlayer:open-network-evidence', { detail: { snapshot_id: snapshotId, ref, trigger: button } }))); host.append(button); }
  function omittedNote(host, omitted, label) { if (omitted > 0) host.append(node('p', 'topology-omitted-note', `${omitted} additional ${label} omitted from this view.`)); }
  function boundedArray(host, values, limit, label, render) { const items = Array.isArray(values) ? values : []; items.slice(0, limit).forEach(render); omittedNote(host, Math.max(0, items.length - limit), label); }
  function evidenceRefs(snapshotId, refs, host, title) { if (!Array.isArray(refs) || !refs.length) return; const row = node('div', 'topology-source-row'); row.append(node('span', 'topology-meta-label', title)); refs.slice(0, 6).forEach((entry) => { const ref = typeof entry === 'object' && entry ? entry.ref || entry.id : entry; const sid = typeof entry === 'object' && entry && entry.snapshot_id || snapshotId; const owner = (state.event && state.event.snapshots || []).map((item) => item.snapshot).find((snapshot) => snapshot && snapshot.id === sid); const evidence = owner && (owner.evidence || []).find((item) => item.id === ref); const label = evidence ? `${String(evidence.kind || 'Source').replaceAll('_', ' ')} · lines ${evidence.start_line}–${evidence.end_line}` : 'Open source'; sourceButton(sid, ref, label, row); }); omittedNote(row, Math.max(0, refs.length - 6), 'source references'); if (row.childElementCount > 1) host.append(row); }
  function collectOwnedEvidence(snapshot, projected, hops) {
    if (!snapshot || !snapshot.id || !Array.isArray(snapshot.evidence)) return [];
    const known = new Map(snapshot.evidence.map((entry) => [String(entry.id),entry])); const refs = new Set();
    const collect = (values, fallbackOwner) => {
      if (!Array.isArray(values)) return;
      values.forEach((entry) => { const ref = entry && typeof entry === 'object' ? entry.ref || entry.id : entry; const owner = entry && typeof entry === 'object' && entry.snapshot_id || fallbackOwner; if (ref !== undefined && ref !== null && String(owner) === String(snapshot.id) && known.has(String(ref))) refs.add(String(ref)); });
    };
    (projected && projected.interfaces || []).forEach((iface) => collect(iface && iface.evidence_refs,snapshot.id));
    const path = snapshot.path || {};
    ['interfaces','routes','nat_rules','translations','policies'].forEach((field) => (Array.isArray(path[field]) ? path[field] : []).forEach((item) => collect(item && item.evidence_refs,snapshot.id)));
    collect(path.evidence_refs,snapshot.id);
    (hops || []).forEach((hop) => {
      if (String(hop.snapshot_id || '') !== String(snapshot.id)) return;
      const pathResult = hop.path || {}; collect(hop.evidence || pathResult.evidence_refs,hop.snapshot_id);
      (Array.isArray(pathResult.stages || pathResult.hops) ? pathResult.stages || pathResult.hops : []).forEach((stage) => collect(stage && (stage.evidence_refs || stage.evidence),hop.snapshot_id));
    });
    return [...refs].map((ref) => ({ ref, evidence: known.get(ref) }));
  }
  function renderDeviceList() {
    if (!state.deviceList) return;
    const query = (state.search.value || '').trim().toLocaleLowerCase(); const nodes = getNodes().filter((item) => matchesFacetFilters(item) && deviceSearchText(item).includes(query));
    state.deviceList.replaceChildren(); state.deviceList.setAttribute('aria-label', query ? 'Matching devices' : 'Devices');
    if (!nodes.length) state.deviceList.append(node('span', 'topology-no-results', query ? 'No matching devices' : 'No devices'));
    nodes.forEach((item) => { const snapshot = getSnapshot(item.node_id); const button = node('button', 'topology-device', item.node_id); button.type = 'button'; button.dataset.nodeId = item.node_id; button.setAttribute('aria-label', `${item.node_id}${snapshot && snapshot.hostname ? ` · ${snapshot.hostname}` : ''}${snapshot && snapshot.vendor ? ` · ${snapshot.vendor}` : ''}${snapshot && snapshot.context ? ` · ${snapshot.context}` : ''}`); button.setAttribute('aria-pressed', String(state.selected === item.node_id)); button.addEventListener('click', () => selectNode(item.node_id, true, true)); state.deviceList.append(button); });
    state.svg.querySelectorAll('.topology-node').forEach((item) => { const match = nodes.some((candidate) => candidate.node_id === item.dataset.nodeId); item.classList.toggle('topology-search-dim', Boolean(query && !match)); item.classList.toggle('topology-search-match', Boolean(query && match)); });
  }
  function renderInspector(nodeId) {
    const snapshot = getSnapshot(nodeId); const entry = (state.event && state.event.snapshots || []).find((item) => item && item.node_id === nodeId); const hops = [...getHops('forward'), ...getHops('return')].filter((hop) => hop.node_id === nodeId);
    const panel = node('div', 'topology-inspector-content'); panel.dataset.inspectorKind = 'device'; const heading = node('div', 'topology-inspector-heading'); heading.append(node('div', 'topology-eyebrow', 'DEVICE INSPECTOR'), node('h4', '', nodeId)); panel.append(heading);
    const projected = getProjectedNode({ node_id: nodeId });
    const annotationKey = roleAnnotationKey(nodeId); const presentation = node('label','topology-role-annotation','Display icon'); const roleSelect = node('select','topology-role-select'); roleSelect.setAttribute('aria-label','Display icon annotation');
    [['','Use projected icon'],['firewall','Firewall'],['router','Router'],['switch','Switch'],['host','Host'],['server','Server'],['network','Network device']].forEach(([value,label]) => { const option = node('option','',label); option.value = value; roleSelect.append(option); }); roleSelect.value = state.roleAnnotations && state.roleAnnotations[annotationKey] || (projected.role_source === 'display_annotation' ? projected.icon : '') || ''; roleSelect.addEventListener('change', () => { const role = roleSelect.value; state.roleAnnotations = state.roleAnnotations || {}; if (role) state.roleAnnotations[annotationKey] = role; else delete state.roleAnnotations[annotationKey]; document.dispatchEvent(new CustomEvent('graphlayer:topology-role', { detail: { node_id: nodeId, role: role || null } })); if (renderGraph()) renderInspector(nodeId); }); presentation.append(roleSelect, node('span','topology-role-note','Display annotation only; does not change the saved model.')); panel.append(presentation);
    const summary = node('dl', 'topology-summary'); const add = (label, value) => { summary.append(node('dt', '', label), node('dd', '', safeText(value))); };
    add('Vendor', snapshot && snapshot.vendor); add('Context', snapshot && snapshot.context); add('Hostname', snapshot && snapshot.hostname); add('Snapshot', snapshot && snapshot.id || entry && entry.snapshot_id);
    add('Sanitized configuration SHA-256', snapshot && snapshot.sanitized_sha256); panel.append(summary);
    const topologyFacts = node('section','topology-inspector-section'); topologyFacts.append(node('h5','','Projected topology attributes'));
    [['Role source',projected.role_source],['Zone',projected.zone],['Subnet',projected.subnet],['VRF',projected.vrf],['VDOM',projected.vdom]].forEach(([label,value]) => topologyFacts.append(node('p','topology-config-item',`${label} · ${safeText(value)}`)));
    boundedArray(topologyFacts,projected.interfaces,12,'projected interfaces',(iface) => { topologyFacts.append(node('p','topology-config-item',`${iface.name || 'Interface'}${iface.address ? ` · ${iface.address}` : ''}${iface.subnet ? ` · subnet ${iface.subnet}` : ''}${iface.zone ? ` · zone ${iface.zone}` : ''}${iface.vrf ? ` · VRF ${iface.vrf}` : ''}${iface.vdom ? ` · VDOM ${iface.vdom}` : ''}`)); evidenceRefs(snapshot && snapshot.id,iface.evidence_refs,topologyFacts,`Interface ${iface.name || ''} sources`); });
    const topologyGroup = node('details','topology-inspector-group'); topologyGroup.dataset.group = 'topology'; topologyGroup.open = false; topologyGroup.append(node('summary','','Projected topology attributes'),topologyFacts); panel.append(topologyGroup);
    const path = snapshot && snapshot.path; const facts = node('section', 'topology-inspector-section'); facts.append(node('h5', '', 'Saved configuration model'));
    if (!path || typeof path !== 'object') facts.append(node('p', 'topology-config-status', 'Routing, NAT, and interface model unavailable for this snapshot.'));
    else {
      const count = (value) => Array.isArray(value) ? String(value.length) : 'unavailable';
      facts.append(node('p', 'topology-config-status', `Interfaces ${count(path.interfaces)} · routes ${count(path.routes)} · ASA NAT rules ${count(path.nat_rules)} · translation entries ${count(path.translations)} · policies ${count(path.policies)} · warnings ${count(path.warnings)}`));
      facts.append(node('p', 'topology-config-status', `Routing coverage ${path.routing_complete === true ? 'complete' : path.routing_complete === false ? 'incomplete' : 'unavailable'} · NAT coverage ${path.nat_complete === true ? 'complete' : path.nat_complete === false ? 'incomplete' : 'unavailable'}`));
      boundedArray(facts, path.warnings, 8, 'warnings', (warning) => facts.append(node('p', 'topology-config-item', `Coverage uncertainty · ${warning.code || 'Warning'}${warning.line ? ` · line ${warning.line}` : ''}: ${warning.message || 'Configuration could not be fully modeled'}`)));
      boundedArray(facts, path.interfaces, 12, 'interfaces', (iface) => { const line = node('p', 'topology-config-item', `${iface.name || iface.physical || 'Interface'}${iface.physical ? ` · ${iface.physical}` : ''}${iface.address ? ` · ${iface.address}` : ''} · ${iface.resolved === false ? 'unresolved' : iface.shutdown ? 'shutdown' : 'configured'}`); facts.append(line); evidenceRefs(snapshot && snapshot.id, iface.evidence_refs, facts, `Interface ${iface.name || iface.physical || ''}`); });
      boundedArray(facts, path.routes, 8, 'routes', (route) => { const line = node('p', 'topology-config-item', `Route ${route.prefix || 'unknown'} via ${route.interface || 'unknown'}${route.next_hop ? ` · next hop ${route.next_hop}` : ''} · ${route.kind || 'unknown'}`); facts.append(line); evidenceRefs(snapshot && snapshot.id, route.evidence_refs, facts, `Route ${route.prefix || ''}`); });
      boundedArray(facts, path.nat_rules, 8, 'ASA NAT rules', (nat) => { const line = node('p', 'topology-config-item', `ASA NAT · ${nat.real_prefix || 'unknown'} → ${nat.mapped_prefix || 'unknown'} · ${nat.real_interface || '?'} → ${nat.mapped_interface || '?'}`); facts.append(line); evidenceRefs(snapshot && snapshot.id, nat.evidence_refs, facts, 'ASA NAT rule'); });
      boundedArray(facts, path.translations, 8, 'translation entries', (translation) => { const line = node('p', 'topology-config-item', `Translation · ${modelValue(translation.kind)} · from ${modelValue(translation.from)} → to ${modelValue(translation.to)} · ${modelValue(translation.original_prefix)} → ${modelValue(translation.translated_prefix)}${translation.policy_id ? ` · policy ${modelValue(translation.policy_id)}` : ''}`); facts.append(line); evidenceRefs(snapshot && snapshot.id, translation.evidence_refs, facts, 'Translation sources'); });
      boundedArray(facts, path.policies, 8, 'policies', (policy) => { const line = node('p', 'topology-config-item', `Policy ${modelValue(policy.id)} · from ${modelValue(policy.from)} → to ${modelValue(policy.to)} · ${modelValue(policy.match && policy.match.source)} → ${modelValue(policy.match && policy.match.destination)} · ${modelValue(policy.match && policy.match.protocol)} · source port ${modelValue(policy.match && policy.match.source_port)} · destination port ${modelValue(policy.match && policy.match.destination_port)} · action ${modelValue(policy.match && policy.match.action)}`); facts.append(line); evidenceRefs(snapshot && snapshot.id, policy.evidence_refs, facts, 'Policy sources'); });
      if (snapshot && snapshot.id) evidenceRefs(snapshot.id, path.evidence_refs, facts, 'Path model sources');
    }
    const factsGroup = node('details','topology-inspector-group'); factsGroup.dataset.group = 'configuration'; factsGroup.open = false; const factsTitle = node('summary','', 'Saved configuration model'); factsGroup.append(factsTitle, facts); panel.append(factsGroup);
    const routeContext = node('section', 'topology-inspector-section'); routeContext.append(node('h5', '', state.event && state.event.mode === 'draft' ? 'Analysis status' : 'Selected path context'));
    const branch = getBranch(); const returnCandidate = getReturn();
    if (state.event && state.event.mode === 'draft') routeContext.append(node('p', 'topology-config-status', 'This is a declared topology draft. The request has not been analyzed; no path disposition or hop result is available.'));
    else if (state.event && state.event.mode === 'trace') {
      const trace = getTraceResult() || {}; const forward = trace.forward || {}; const returned = trace.return || {};
      routeContext.append(node('p', 'topology-config-status', `Forward trace ${forward.disposition || 'unavailable'} · return trace ${returned.disposition || 'unavailable'}${returned.session_status ? ` · session ${returned.session_status}` : ''}${returned.asymmetric === true ? ' · asymmetric route' : ''}`));
    } else routeContext.append(node('p', 'topology-config-status', `Forward branch ${branch && branch.disposition || 'unavailable'} · return candidate ${returnCandidate && returnCandidate.disposition || 'unavailable'} · session ${returnCandidate && returnCandidate.session_status || 'unknown'}${returnCandidate && returnCandidate.asymmetric ? ' · asymmetric route' : ''}`));
    if (!hops.length && !(state.event && state.event.mode === 'draft')) routeContext.append(node('p', 'topology-config-status', 'This declared device has no hop in the selected forward or return candidate.'));
    boundedArray(routeContext, hops, 12, 'selected path hops', (hop) => { const direction = getHops('forward').includes(hop) ? 'Forward' : 'Return'; const p = hop.path || {}; routeContext.append(node('div', 'topology-hop-card', `${direction} · ${p.disposition || 'unknown'} · ingress ${p.ingress_interface || p.ingress || 'unavailable'} · egress ${p.egress_interface || 'unavailable'}${p.selected_route ? ` · route ${p.selected_route.prefix || '?'} via ${p.selected_route.interface || '?'}${p.selected_route.next_hop ? ` next hop ${p.selected_route.next_hop}` : ''}` : ''}`));
      const tuple = (label, value) => { if (value) routeContext.append(node('p', 'topology-tuple', `${label}: ${[value.source, value.destination].filter(Boolean).join(' → ')} · ${value.protocol || ''}${value.source_port ? `:${value.source_port}` : ''}${value.destination_port ? ` → :${value.destination_port}` : ''}`)); };
      tuple('Original tuple', p.original_flow); tuple('Policy tuple', p.policy_flow); tuple('Egress tuple', p.egress_flow || p.final_flow);
      boundedArray(routeContext, p.stages || p.hops, 10, 'path stages', (stage) => { routeContext.append(node('p', 'topology-config-item', `${stage.name || stage.kind || 'Decision'} · ${stage.status || stage.disposition || 'unavailable'}: ${stage.detail || stage.reason || 'No explanation returned'}`)); evidenceRefs(hop.snapshot_id, stage.evidence_refs || stage.evidence, routeContext, `${stage.name || stage.kind || 'Decision'} sources`); }); evidenceRefs(hop.snapshot_id, hop.evidence || p.evidence_refs, routeContext, `${direction} hop sources`);
    });
    routeContext.append(node('p', 'topology-runtime-unavailable', 'Runtime session state, packet captures, counters, and live forwarding observations are unavailable.')); const pathGroup = node('details','topology-inspector-group'); pathGroup.dataset.group = 'path'; pathGroup.open = false; pathGroup.append(node('summary','',state.event && state.event.mode === 'draft' ? 'Analysis status' : 'Selected path context'), routeContext); panel.append(pathGroup);
    const evidenceContent = node('section','topology-inspector-section'); evidenceContent.append(node('h5','','Saved source references'));
    const ownedEvidence = collectOwnedEvidence(snapshot,projected,hops); const evidenceRow = node('div','topology-source-row'); evidenceRow.append(node('span','topology-meta-label','Configuration and selected path sources'));
    ownedEvidence.slice(0,18).forEach(({ref,evidence}) => sourceButton(snapshot && snapshot.id,ref,`${String(evidence.kind || 'Source').replaceAll('_',' ')} · lines ${evidence.start_line}–${evidence.end_line}`,evidenceRow));
    if (ownedEvidence.length > 18) omittedNote(evidenceRow,ownedEvidence.length - 18,'source references');
    if (evidenceRow.childElementCount > 1) evidenceContent.append(evidenceRow); else evidenceContent.append(node('p','topology-config-status','No owned source references are available for this snapshot or its selected path.'));
    const evidenceGroup = node('details','topology-inspector-group'); evidenceGroup.dataset.group = 'evidence'; evidenceGroup.open = false; evidenceGroup.append(node('summary','','Source evidence'),evidenceContent); panel.append(evidenceGroup);
    if (snapshot && snapshot.id) { const identity = node('p','topology-inspector-identity',`Snapshot ${snapshot.id}`); identity.title = snapshot.id; heading.append(identity); }
    const metrics = node('div','topology-inspector-metrics'); metrics.setAttribute('aria-label','Device detail shortcuts');
    const detailMetrics = [['Interfaces',Array.isArray(path && path.interfaces) ? path.interfaces.length : null,factsGroup],['Routes',Array.isArray(path && path.routes) ? path.routes.length : null,factsGroup],['Path hops',hops.length,pathGroup],['Sources',ownedEvidence.length,evidenceGroup]];
    detailMetrics.forEach(([label,value,group]) => { const shortcut = node('button','topology-inspector-metric'); shortcut.type = 'button'; shortcut.setAttribute('aria-label',`Inspect ${label.toLowerCase()}`); shortcut.append(node('strong','',value === null ? '—' : value),node('span','',label)); shortcut.addEventListener('click',() => { group.open = true; const summary = group.querySelector('summary'); if (summary) { summary.focus({preventScroll:true}); group.scrollIntoView({block:'nearest',behavior:'instant'}); } }); metrics.append(shortcut); }); panel.insertBefore(metrics,presentation);
    state.inspector.replaceChildren(panel);
  }
  function selectLink(index) {
    const link = (getTopology().links || getRequest().links || [])[index]; if (!link) return; state.selectedLink = index;
    setInspectorCollapsed(false);
    state.svg.querySelectorAll('.topology-link').forEach((item) => { const selected = Number(item.dataset.linkIndex) === index; item.classList.toggle('topology-link-selected',selected); item.setAttribute('aria-pressed',String(selected)); }); state.svg.querySelectorAll('.topology-node').forEach((item) => { item.classList.remove('topology-selected'); item.setAttribute('aria-pressed','false'); });
    const panel = node('div','topology-inspector-content'); panel.dataset.inspectorKind = 'link'; const heading = node('div','topology-inspector-heading'); heading.append(node('div','topology-eyebrow','LINK INSPECTOR'),node('h4','',`${link.a.node_id} ↔ ${link.b.node_id}`)); panel.append(heading);
    const summary = node('dl','topology-summary'); const add = (label,value) => summary.append(node('dt','',label),node('dd','',safeText(value)));
    add('Endpoint A',link.a.node_id); add('Port A',link.a.interface); add('Endpoint B',link.b.node_id); add('Port B',link.b.interface); add('Provenance',link.provenance || (link.synthetic ? 'query endpoint' : 'declared topology'));
    if (link.direction) add('Direction',link.direction); if (link.direction_source) add('Direction source',link.direction_source); panel.append(summary);
    const evidence = node('section','topology-inspector-section'); evidence.append(node('h5','','Link evidence'));
    const ownerA = getSnapshot(link.a.node_id), ownerB = getSnapshot(link.b.node_id);
    if (Array.isArray(link.evidence_refs)) evidenceRefs(link.snapshot_id || ownerA && ownerA.id || ownerB && ownerB.id,link.evidence_refs,evidence,'Link sources');
    if (!evidence.querySelector('.topology-source-button')) evidence.append(node('p','topology-config-status','No link evidence references were supplied.'));
    const evidenceGroup = node('details','topology-inspector-group'); evidenceGroup.dataset.group = 'evidence'; evidenceGroup.open = true; evidenceGroup.append(node('summary','','Link evidence'),evidence); panel.append(evidenceGroup); state.inspector.replaceChildren(panel);
  }
  function setInspectorCollapsed(collapsed) {
    if (!state.body) return;
    state.inspectorCollapsed = Boolean(collapsed); state.body.classList.toggle('topology-inspector-collapsed',state.inspectorCollapsed);
    if (state.inspectorToggle) { state.inspectorToggle.setAttribute('aria-expanded',String(!state.inspectorCollapsed)); state.inspectorToggle.setAttribute('aria-label',state.inspectorCollapsed ? 'Show selected device details' : 'Hide selected device details'); state.inspectorToggle.textContent = state.inspectorCollapsed ? 'Details' : 'Hide details'; }
  }
  function setFullscreen(active) {
    if (!state.root || !state.fullscreenToggle || state.fullscreen === Boolean(active)) return;
    state.fullscreen = Boolean(active);
    if (state.fullscreen) {
      state.fullscreenFocus = document.activeElement;
      state.savedBodyOverflow = document.body.style.overflow;
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = state.savedBodyOverflow || '';
      state.savedBodyOverflow = null;
    }
    state.root.classList.toggle('topology-fullscreen-active',state.fullscreen);
    state.fullscreenToggle.setAttribute('aria-pressed',String(state.fullscreen));
    state.fullscreenToggle.setAttribute('aria-label',state.fullscreen ? 'Exit full-screen topology' : 'Expand topology canvas');
    state.fullscreenToggle.textContent = state.fullscreen ? 'Exit full screen' : 'Full screen';
    const desiredState = state.fullscreen;
    requestAnimationFrame(() => {
      if (state.fullscreen !== desiredState) return;
      if (state.svg.querySelector('.topology-node') && renderGraph()) state.fit();
      if (!state.fullscreen) {
        const target = state.fullscreenFocus && state.fullscreenFocus.isConnected ? state.fullscreenFocus : state.fullscreenToggle;
        state.fullscreenFocus = null; target.focus({ preventScroll: true });
      }
    });
  }
  function selectNode(nodeId, focus, revealInspector) {
    if (!nodeId) return; state.selected = nodeId; state.selectedLink = -1; state.svg.querySelectorAll('.topology-link').forEach((item) => { item.classList.remove('topology-link-selected'); item.setAttribute('aria-pressed','false'); }); state.svg.querySelectorAll('.topology-node').forEach((item) => { const selected = item.dataset.nodeId === nodeId; item.classList.toggle('topology-selected', selected); item.setAttribute('aria-pressed', String(selected)); });
    if (revealInspector) setInspectorCollapsed(false);
    state.deviceList.querySelectorAll('.topology-device').forEach((button) => button.setAttribute('aria-pressed', String(button.dataset.nodeId === nodeId)));
    renderInspector(nodeId); if (focus) focusNode(nodeId);
  }
  function focusNode(nodeId) {
    const target = [...state.svg.querySelectorAll('.topology-node')].find((item) => item.dataset.nodeId === nodeId); const card = target && target.querySelector('.topology-node-card'); if (!target || !card || !state.base) return;
    const centerX = Number(card.getAttribute('x')) + Number(card.getAttribute('width')) / 2; const centerY = Number(card.getAttribute('y')) + Number(card.getAttribute('height')) / 2; state.scale = Math.max(1.5, Math.min(4, state.scale)); const width = state.base.width / state.scale; const height = state.base.height / state.scale; state.x = centerX - width / 2; state.y = centerY - height / 2; state.applyZoom(); target.focus({ preventScroll: true });
  }
  function showTopologyStatus(status, message) {
    setup(); if (!state.root) return;
    const blocked = status !== 'ready'; const busy = status === 'loading';
    [state.toolbar,state.resultControls,state.traceBadge,state.filtersHost,state.deviceList,state.stage,state.inspector].forEach((part) => { if (!part) return; part.inert = blocked; part.setAttribute('aria-busy',String(busy)); });
    if (!blocked) return;
    if (busy) {
      if (!state.loadingView) state.loadingView = { identity: state.identity, selected: state.selected, selectedLink: state.selectedLink, scale: state.scale, x: state.x, y: state.y, base: state.base };
    } else {
      state.loadingView = null; state.base = null; state.scale = 1; state.x = 0; state.y = 0; state.selected = ''; state.selectedLink = -1;
    }
    state.svg.replaceChildren(); state.svg.setAttribute('viewBox','0 0 1000 440'); state.svg.setAttribute('aria-label',`${status} topology view`);
    const group = svgElement('g',{class:`topology-state-message topology-state-${status}`},state.svg);
    const title = svgElement('text',{x:500,y:205,'text-anchor':'middle',class:'topology-state-title'},group); title.textContent = status === 'loading' ? 'Preparing topology view…' : status === 'incomplete' ? 'Topology view unavailable' : 'Topology view could not be loaded';
    const detail = svgElement('text',{x:500,y:234,'text-anchor':'middle',class:'topology-state-detail'},group); detail.textContent = message || (status === 'incomplete' ? 'Select a saved snapshot for each declared device.' : 'The topology service returned an error.');
    state.caption.textContent = status === 'loading' ? 'Loading projected topology…' : message || 'No current topology projection is available.';
    state.deviceList.replaceChildren(); state.inspector.replaceChildren(node('div','topology-inspector-empty',message || 'Topology details are unavailable.'));
  }
  function onTopologyStatus(event) { const detail = event && event.detail || {}; showTopologyStatus(detail.status || 'error',detail.message || ''); }
  function update(event) {
    setup(); if (!state.root || !event || !event.detail) return;
    const request = event.detail.request || event.detail.exploration && event.detail.exploration.request || {};
    const nextIdentity = event.detail.mode === 'draft' ? `draft:${JSON.stringify(request)}` : event.detail.mode === 'trace' ? `trace:${event.detail.investigation && event.detail.investigation.id || JSON.stringify(request)}` : `result:${event.detail.exploration && event.detail.exploration.id || JSON.stringify(request)}`;
    state.event = event.detail; indexPresentation();
    renderResultControls(event.detail);
    const changedIdentity = state.identity !== nextIdentity; state.identity = nextIdentity;
    const loadingView = state.loadingView; state.loadingView = null;
    if (changedIdentity) { state.selected = ''; state.selectedLink = -1; }
    showTopologyStatus('ready','');
    if (!renderGraph()) return;
    if (changedIdentity) { state.selected = ''; state.fit(); }
    else if (loadingView && loadingView.identity === nextIdentity && loadingView.base) { state.selected = loadingView.selected; state.selectedLink = loadingView.selectedLink; state.scale = loadingView.scale; state.x = loadingView.x; state.y = loadingView.y; state.applyZoom(); }
    else state.applyZoom();
    state.svg.classList.toggle('topology-hide-forward', !state.forwardToggle.checked); state.svg.classList.toggle('topology-hide-return', !state.returnToggle.checked);
    const trafficAvailable = Boolean(getTopology().traffic && getTopology().traffic.available === true); state.trafficStatus.dataset.available = String(trafficAvailable); state.trafficStatus.textContent = trafficAvailable ? 'Source-provided traffic data available' : 'Traffic observations unavailable';
    if (event.detail.mode === 'draft') {
      state.heading.textContent = 'Declared topology draft'; state.legend.textContent = 'Operator supplied links and device snapshots. Analysis has not run; runtime observations are unavailable.';
      state.caption.textContent = 'Declared topology draft · analysis not run · runtime observations unavailable'; state.svg.setAttribute('aria-label', 'Declared network topology draft. Analysis has not run.');
    } else if (event.detail.mode === 'trace') {
      state.heading.textContent = 'Network path trace'; state.legend.textContent = 'Operator supplied links with the saved configured forward and return trace. Runtime observations are unavailable.';
      state.caption.textContent = 'Declared links · configured forward/return trace · runtime observations unavailable'; state.svg.setAttribute('aria-label', 'Declared network topology with configured forward and return trace.');
    } else {
      state.heading.textContent = 'Declared topology and selected paths'; state.legend.textContent = 'Operator supplied links with selected configured forward and return paths. This is route analysis, not live traffic observation.';
      state.caption.textContent = `Declared links · selected branch ${Number(event.detail.branchIndex) + 1} · configured forward/return paths · runtime observations unavailable`; state.svg.setAttribute('aria-label', 'Declared network topology with selected branch overlay');
    }
    renderDeviceList();
    if (state.selected && getNodes().some((item) => item.node_id === state.selected)) selectNode(state.selected, false);
    else if (getNodes().length) selectNode(getNodes()[0].node_id, false);
    else { state.selected = ''; state.inspector.replaceChildren(node('div', 'topology-inspector-empty', 'Add a device and select its saved snapshot to inspect configuration evidence.')); }
  }
  document.addEventListener('graphlayer:topology-updated', update);
  document.addEventListener('graphlayer:topology-status', onTopologyStatus);
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', setup, { once: true }); else setup();
})();
