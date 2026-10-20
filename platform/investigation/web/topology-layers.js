(() => {
  'use strict';

  const state = { root: null, csrf: '', dataset: null, projection: null, view: 'l3', network: '', zone: '', search: '', offset: 0, limit: 25, source: '', destination: '', busy: false, seq: 0, timer: 0, controller: null, scale: 1, x: 0, y: 0, pan: null, selected: null, fullscreenActive: false, fullscreenFocus: null, bodyOverflow: null };
  const NS = 'http://www.w3.org/2000/svg';
  const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text !== undefined) e.textContent = String(text); return e; };
  const button = (id, label, cls = 'tl-button') => { const b = el('button', cls, label); b.type = 'button'; if (id) b.id = id; return b; };
  const field = (label, id, type = 'text') => { const wrap = el('label', 'tl-field', label); const input = el('input'); input.id = id; input.type = type; input.autocomplete = 'off'; wrap.append(input); return { wrap, input }; };
  const svg = (tag, attrs = {}) => { const e = document.createElementNS(NS, tag); for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, String(v)); return e; };
  const arr = (v) => Array.isArray(v) ? v : [];
  const first = (...xs) => xs.find((x) => x !== undefined && x !== null);
  const str = (x, fallback = '') => x === undefined || x === null || x === '' ? fallback : String(x);
  const root = () => state.root || document.getElementById('topology-layers');

  function setup() {
    state.root = root();
    if (!state.root || state.root.dataset.ready) return;
    state.root.dataset.ready = 'true';
    state.root.classList.add('topology-layers');
    const header = el('div', 'tl-heading');
    const title = el('div'); title.append(el('span', 'tl-eyebrow', 'SYNTHETIC SCALE LAB'), el('h1', '', 'Network map'), el('p', '', 'Explore a thousand simulated hosts across routed networks and security zones.'));
    const headActions = el('div', 'tl-head-actions');
    const load = button('tl-load', 'Load 1,000-host lab', 'tl-button tl-primary');
    const exportButton = button('tl-export', 'Export dataset'); exportButton.disabled = true;
    headActions.append(exportButton, load); header.append(title, headActions);
    const summary = el('div', 'tl-summary'); summary.id = 'tl-summary';
    const tabs = el('div', 'tl-tabs'); tabs.setAttribute('role', 'tablist'); tabs.setAttribute('aria-label', 'Topology view');
    const tabDefs = [['l3', 'Layer 3'], ['zone', 'Zones'], ['l2', 'Host & L2']];
    for (const [key, label] of tabDefs) { const b = button(`tl-tab-${key}`, label, 'tl-tab'); b.dataset.view = key; b.setAttribute('role', 'tab'); b.setAttribute('aria-controls', 'tl-view-panel'); b.setAttribute('aria-selected', String(state.view === key)); tabs.append(b); }
    const controls = el('div', 'tl-controls');
    const net = field('NETWORK', 'tl-network'); net.input.placeholder = 'All networks';
    const zone = field('ZONE', 'tl-zone'); zone.input.placeholder = 'All zones';
    const search = field('SEARCH', 'tl-search', 'search'); search.input.placeholder = 'Host, IP, MAC…'; search.input.setAttribute('aria-label', 'Search hosts and networks');
    controls.append(net.wrap, zone.wrap, search.wrap);
    const canvasHead = el('div', 'tl-canvas-head');
    const graphTitle = el('strong', '', 'Layer 3 overview');
    const actions = el('div', 'tl-actions');
    const fit = button('tl-fit', 'Fit'); const zoomOut = button('tl-zoom-out', '−', 'tl-button tl-square'); const zoomLabel = el('output', 'tl-zoom', '100%'); zoomLabel.id = 'tl-zoom'; const zoomIn = button('tl-zoom-in', '+', 'tl-button tl-square'); const fullscreen = button('tl-fullscreen', 'Full screen'); fullscreen.setAttribute('aria-pressed', 'false');
    actions.append(fit, zoomOut, zoomLabel, zoomIn, fullscreen); canvasHead.append(graphTitle, actions);
    const graph = el('div', 'tl-graph'); graph.id = 'tl-graph'; graph.tabIndex = 0; graph.setAttribute('role', 'tabpanel'); graph.setAttribute('aria-labelledby', 'tl-tab-l3'); graph.setAttribute('aria-label', 'Interactive network map');
    const drawing = svg('svg', { id: 'tl-svg', role: 'group', 'aria-label': 'Network topology diagram' }); const stage = svg('g', { id: 'tl-stage' }); drawing.append(stage); graph.append(drawing);
    const legend = el('div', 'tl-legend'); legend.setAttribute('aria-label', 'Network map legend');
    for (const [kind, role, label] of [['host', '', 'Host'], ['network', '', 'Network'], ['device', 'router', 'Router'], ['device', 'switch', 'Switch'], ['device', 'firewall', 'Firewall'], ['zone', '', 'Zone']]) { const item = el('span', 'tl-legend-item'); const iconSvg = svg('svg', { viewBox: '-12 -12 24 24', 'aria-hidden': 'true' }); const icon = makeIcon(kind, { role }); icon.setAttribute('transform', ''); iconSvg.append(icon); item.append(iconSvg, el('span', '', label)); legend.append(item); }
    const caption = el('div', 'tl-caption'); caption.id = 'tl-caption'; caption.setAttribute('role', 'status');
    const pager = el('div', 'tl-pager'); pager.id = 'tl-pager';
    const prev = button('tl-prev', '← Previous'); const pageLabel = el('span', ''); pageLabel.id = 'tl-page-label'; const next = button('tl-next', 'Next →'); pager.append(prev, pageLabel, next);
    const lower = el('div', 'tl-lower');
    const trace = el('section', 'tl-trace'); trace.setAttribute('aria-labelledby', 'tl-trace-title'); const traceTitle = el('h3', '', 'Trace a host path'); traceTitle.id = 'tl-trace-title'; trace.append(traceTitle);
    const traceCopy = el('p', 'tl-muted', 'Choose two hosts from the current projection, then ask Rust to trace the Layer 2 path.'); trace.append(traceCopy);
    const source = field('SOURCE HOST', 'tl-source'); source.input.placeholder = 'Select a lab host'; source.input.setAttribute('list', 'tl-host-options');
    const destination = field('DESTINATION HOST', 'tl-destination'); destination.input.placeholder = 'Select a lab host'; destination.input.setAttribute('list', 'tl-host-options');
    const options = el('datalist'); options.id = 'tl-host-options'; const traceButton = button('tl-trace-button', 'Trace L2 path', 'tl-button tl-primary');
    const traceRow = el('div', 'tl-trace-row'); traceRow.append(source.wrap, destination.wrap, traceButton); trace.append(traceRow, options);
    const traceStatus = el('div', 'tl-trace-status'); traceStatus.id = 'tl-trace-status'; traceStatus.setAttribute('aria-live', 'polite'); trace.append(traceStatus);
    const inspector = el('aside', 'tl-inspector'); inspector.id = 'tl-inspector'; inspector.setAttribute('aria-label', 'Selection details'); inspector.append(el('div', 'tl-empty', 'Select a network, zone, or host to inspect its details.'));
    lower.append(trace, inspector);
    const sourceDrawer = el('details', 'tl-sources'); sourceDrawer.id = 'tl-sources'; const sourceSummary = el('summary', '', 'Auxiliary source evidence'); const sourceBody = el('div', 'tl-source-body'); sourceBody.id = 'tl-source-body'; sourceDrawer.append(sourceSummary, sourceBody);
    const note = el('p', 'tl-note', 'All lab devices and observations are synthetic. Configuration projections do not establish live reachability or runtime state.');
    state.root.replaceChildren(header, summary, tabs, controls, canvasHead, graph, legend, caption, pager, lower, sourceDrawer, note);
    Object.assign(state, { load, exportButton, summary, tabs, net: net.input, zoneInput: zone.input, searchInput: search.input, graphTitle, drawing, stage, graph, legend, caption, pager, prev, next, pageLabel, inspector, sourceField: source, destinationField: destination, options, traceButton, traceStatus, sourceBody, fullscreenButton: fullscreen, zoomLabel });
    load.addEventListener('click', loadDataset);
    exportButton.addEventListener('click', exportDataset);
    tabs.addEventListener('click', (e) => { const b = e.target.closest('[data-view]'); if (b) chooseView(b.dataset.view); });
    tabs.addEventListener('keydown', (e) => { const active = e.target.closest('[data-view]'); if (!active) return; const buttons = [...tabs.querySelectorAll('[data-view]')]; let i = buttons.indexOf(active); if (e.key === 'ArrowRight') i = (i + 1) % buttons.length; else if (e.key === 'ArrowLeft') i = (i - 1 + buttons.length) % buttons.length; else if (e.key === 'Home') i = 0; else if (e.key === 'End') i = buttons.length - 1; else return; e.preventDefault(); chooseView(buttons[i].dataset.view); buttons[i].focus(); });
    [net.input, zone.input].forEach((input) => input.addEventListener('change', () => { state.network = net.input.value.trim(); state.zone = zone.input.value.trim(); state.offset = 0; requestProjection(); }));
    search.input.addEventListener('input', () => { state.search = search.input.value.trim(); state.offset = 0; clearTimeout(state.timer); state.timer = setTimeout(requestProjection, 180); });
    prev.addEventListener('click', () => { if (state.offset >= state.limit) { state.offset -= state.limit; requestProjection(); } });
    next.addEventListener('click', () => { state.offset += state.limit; requestProjection(); });
    fit.addEventListener('click', fitGraph); zoomIn.addEventListener('click', () => setZoom(state.scale * 1.2)); zoomOut.addEventListener('click', () => setZoom(state.scale / 1.2));
    fullscreen.addEventListener('click', toggleFullscreen);
    graph.addEventListener('wheel', (e) => { e.preventDefault(); setZoom(state.scale * (e.deltaY < 0 ? 1.12 : 1 / 1.12)); }, { passive: false });
    graph.addEventListener('pointerdown', (e) => { if (e.button !== 0 || e.target.closest('.tl-node')) return; state.pan = { x: e.clientX, y: e.clientY, ox: state.x, oy: state.y }; graph.setPointerCapture(e.pointerId); graph.classList.add('is-panning'); });
    graph.addEventListener('pointermove', (e) => { if (!state.pan) return; state.x = state.pan.ox + e.clientX - state.pan.x; state.y = state.pan.oy + e.clientY - state.pan.y; applyTransform(); });
    const stopPan = () => { state.pan = null; graph.classList.remove('is-panning'); }; graph.addEventListener('pointerup', stopPan); graph.addEventListener('pointercancel', stopPan);
    drawing.addEventListener('click', (e) => { const item = e.target.closest('.tl-node'); if (item) selectItem(item.__item); });
    drawing.addEventListener('keydown', (e) => { if (!['Enter', ' '].includes(e.key)) return; const item = e.target.closest('.tl-node'); if (item) { e.preventDefault(); selectItem(item.__item); } });
    traceButton.addEventListener('click', requestPath);
    source.input.addEventListener('input', () => { state.source = ''; clearPathDisplay(); }); destination.input.addEventListener('input', () => { state.destination = ''; clearPathDisplay(); });
    source.input.addEventListener('change', () => { state.source = findHostId(source.input.value); }); destination.input.addEventListener('change', () => { state.destination = findHostId(destination.input.value); });
    window.addEventListener('resize', () => applyTransform());
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && state.fullscreenActive) toggleFullscreen(); });
    window.addEventListener('graphlayer:session', (e) => setSession(e.detail && e.detail.csrf));
    drawTabs(); updateControls();
  }

  function setSession(csrf) { const next = str(csrf); if (next !== state.csrf && state.controller) { state.seq += 1; state.controller.abort(); state.controller = null; state.busy = false; } state.csrf = next; if (!state.csrf && state.stage) clearProjectionUI('Waiting for the local session…'); updateControls(); }
  function updateControls() { if (state.load) state.load.disabled = !state.csrf || state.busy; if (state.traceButton) state.traceButton.disabled = !state.csrf || state.busy || !state.dataset; }
  async function post(url, body, signal) {
    const response = await fetch(url, { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': state.csrf }, body: JSON.stringify(body), signal });
    let payload; try { payload = await response.json(); } catch (_) { throw new Error(`Request failed (${response.status})`); }
    if (!response.ok) { const error = payload && payload.error; throw new Error(str(error && (error.message || error.code), `Request failed (${response.status})`)); }
    return payload;
  }
  async function loadDataset() {
    if (!state.csrf || state.busy) return;
    const seq = ++state.seq; state.controller?.abort(); const controller = new AbortController(); state.controller = controller;
    state.busy = true; state.dataset = null; state.projection = null; state.view = 'l3'; state.network = ''; state.zone = ''; state.search = ''; state.offset = 0; state.source = ''; state.destination = ''; state.selected = null; state.net.value = ''; state.zoneInput.value = ''; state.searchInput.value = ''; state.sourceField.input.value = ''; state.destinationField.input.value = ''; drawTabs(); clearProjectionUI('Generating synthetic lab dataset…'); state.load.textContent = 'Loading lab…'; updateControls();
    try { const result = await post('/api/sample/topology-layers', {}, controller.signal); if (seq !== state.seq) return; const dataset = result.dataset || result; validateDataset(dataset); state.dataset = dataset; state.offset = 0; state.load.textContent = 'Reload lab dataset'; await requestProjection(); }
    catch (error) { if (seq === state.seq && error.name !== 'AbortError') showError(error); }
    finally { if (seq === state.seq) { state.busy = false; state.load.textContent = state.dataset ? 'Reload lab dataset' : 'Load 1,000-host lab'; updateControls(); } }
  }
  function query() { return { dataset: state.dataset, view: state.view, network_id: state.network, zone: state.zone, search: state.search, offset: state.offset, limit: state.limit, source_host_id: state.source, destination_host_id: state.destination }; }
  async function requestProjection() {
    if (!state.csrf || !state.dataset) return;
    const seq = ++state.seq; state.controller?.abort(); const controller = new AbortController(); state.controller = controller;
    const request = query(); state.busy = true; state.projection = null; clearProjectionUI('Updating network map…'); updateControls();
    try { const result = await post('/api/topology-layers', request, controller.signal); if (seq !== state.seq) return null; const projection = result.projection || result; validateProjection(projection); state.projection = projection; render(); return projection; }
    catch (error) { if (seq === state.seq && error.name !== 'AbortError') showError(error); return null; }
    finally { if (seq === state.seq) { state.busy = false; state.controller = null; updateControls(); } }
  }
  function validateDataset(dataset) { if (!isObject(dataset) || dataset.schema_version !== 1 || dataset.synthetic !== true || !Array.isArray(dataset.hosts) || dataset.hosts.length > 10000 || !Array.isArray(dataset.networks) || !Array.isArray(dataset.sources) || dataset.sources.length > 100 || dataset.hosts.some((host) => !isObject(host) || !host.id) || dataset.sources.some((source) => !isObject(source) || !source.id || !Array.isArray(source.lines) || !source.sha256)) throw new Error('The lab response has an unsupported dataset shape.'); }
  function validateProjection(projection) { if (!isObject(projection) || projection.schema_version !== 1 || projection.synthetic !== true || !Array.isArray(projection.nodes) || projection.nodes.length > 140 || !Array.isArray(projection.links) || projection.links.length > 5000 || !Array.isArray(projection.sources) || !isObject(projection.counts) || !isObject(projection.pagination) || projection.nodes.some((item) => !isObject(item) || !idOf(item)) || projection.links.some((edge) => !isObject(edge) || !first(edge.id, edge.from, edge.source_id) || !first(edge.to, edge.target, edge.destination_id))) throw new Error('The topology response has an unsupported or unbounded shape.'); }
  function isObject(value) { return Boolean(value && typeof value === 'object' && !Array.isArray(value)); }
  function clearProjectionUI(message) { if (!state.stage) return; state.stage.replaceChildren(); state.drawing.setAttribute('viewBox', '0 0 1100 600'); state.caption.classList.remove('is-error'); state.caption.textContent = message; state.graphTitle.textContent = ({ l3: 'Layer 3 overview', zone: 'Zone overview', l2: 'Host & Layer 2 detail' })[state.view]; state.graph.setAttribute('aria-labelledby', `tl-tab-${state.view}`); state.pager.hidden = true; state.summary.replaceChildren(); for (const [label, value] of [['Hosts', '—'], ['Networks', '—'], ['Visible elements', '—']]) { const cell = el('div', 'tl-stat'); cell.append(el('span', '', label), el('strong', '', value)); state.summary.append(cell); } state.traceStatus.replaceChildren(); state.sourceBody.replaceChildren(); document.getElementById('tl-sources').open = false; state.options.replaceChildren(); state.exportButton.disabled = !state.dataset; state.inspector.replaceChildren(el('div', 'tl-empty', 'Select a network, zone, or host to inspect its details.')); }
  function showError(error) { state.projection = null; clearProjectionUI(`Could not load the network map: ${str(error && error.message, 'Request failed')}`); state.caption.classList.add('is-error'); }
  function drawTabs() { if (!state.tabs) return; state.tabs.querySelectorAll('[data-view]').forEach((b) => { const active = b.dataset.view === state.view; b.setAttribute('aria-selected', String(active)); b.tabIndex = active ? 0 : -1; }); if (state.graph) state.graph.setAttribute('aria-labelledby', `tl-tab-${state.view}`); }
  function chooseView(view) { if (!['l2', 'l3', 'zone'].includes(view) || view === state.view) return; state.view = view; state.offset = 0; drawTabs(); requestProjection(); }
  function clearPathDisplay() { state.traceStatus.replaceChildren(); state.stage.querySelectorAll('.tl-node.tl-on-path').forEach((node) => node.classList.remove('tl-on-path')); state.stage.querySelectorAll('.tl-edge.tl-path-edge').forEach((edge) => edge.classList.remove('tl-path-edge')); }
  function projectionNodes() { const p = state.projection || {}; return arr(first(p.nodes, p.items, p.vertices, p.hosts)); }
  function projectionEdges() { const p = state.projection || {}; return arr(first(p.edges, p.links, p.connections)); }
  function layoutNodes(nodes, width, view) {
    const positions = new Map();
    if (view === 'zone') { const zones = nodes.filter((item) => typeOf(item) === 'zone'); zones.forEach((item, i) => positions.set(idOf(item), { x: 180 + i * 245, y: 260 })); return positions; }
    if (view === 'l3') {
      const firewalls = nodes.filter((item) => /firewall/.test(str(item.role, '').toLowerCase()));
      const routers = nodes.filter((item) => /router/.test(str(item.role, '').toLowerCase()));
      const networks = nodes.filter((item) => ['network', 'subnet'].includes(typeOf(item)));
      const placeGrid = (items, y, columns, rowGap) => items.forEach((item, i) => { const row = Math.floor(i / columns); const col = i % columns; const rowCount = Math.min(columns, items.length - row * columns); positions.set(idOf(item), { x: width * (col + 1) / (rowCount + 1), y: y + row * rowGap }); });
      placeGrid(firewalls, 75, 4, 90);
      networks.forEach((network, i) => {
        const column = i % 5; const row = Math.floor(i / 5);
        const columns = Math.min(5, networks.length - row * 5);
        const x = width * (column + 1) / (columns + 1); const y = 200 + row * 230;
        positions.set(idOf(network), { x, y: y + 100 });
        const gateway = routers.find((item) => item.network_id === idOf(network));
        if (gateway) positions.set(idOf(gateway), { x, y });
      });
      const remaining = nodes.filter((item) => !positions.has(idOf(item))); placeGrid(remaining, 320, 5, 90);
      return positions;
    }
    const infra = nodes.filter((item) => typeOf(item) !== 'host');
    const hosts = nodes.filter((item) => typeOf(item) === 'host');
    for (const item of infra) {
      const id = idOf(item); const role = str(item.role, '').toLowerCase();
      const match = id.match(/-(\d{2})-([abd])$/); const letter = match && match[2];
      let x = 550; let y = 100;
      if (/router/.test(role)) { x = 550; y = 70; }
      else if (/firewall/.test(role)) { x = id.includes('panos') ? 390 : 710; y = 70; }
      else if (/switch/.test(role)) { x = letter === 'a' ? 300 : letter === 'b' ? 800 : 550; y = letter === 'd' ? 190 : 145; }
      positions.set(id, { x, y });
    }
    hosts.forEach((item, i) => { const col = i % 5; const row = Math.floor(i / 5); positions.set(idOf(item), { x: 100 + col * 225, y: 280 + row * 82 }); });
    return positions;
  }
  function render() {
    state.caption.classList.remove('is-error');
    const p = state.projection || {}; const nodes = projectionNodes(); const edges = projectionEdges();
    const counts = p.counts || {}; const totalHosts = first(counts.hosts, p.total_hosts, p.host_count, '—'); const networks = first(counts.networks, p.network_count, '—'); const shown = first(counts.visible_nodes, p.returned, p.returned_count, nodes.length);
    state.summary.replaceChildren(); for (const [label, value] of [['Hosts', totalHosts], ['Networks', networks], ['Visible elements', shown]]) { const cell = el('div', 'tl-stat'); cell.append(el('span', '', label), el('strong', '', value)); state.summary.append(cell); }
    const viewLabels = { l3: 'Layer 3 overview', zone: 'Zone overview', l2: 'Host & Layer 2 detail' }; state.graphTitle.textContent = viewLabels[state.view];
    state.stage.replaceChildren();
    if (!nodes.length) { state.drawing.setAttribute('viewBox', '0 0 1100 600'); state.caption.textContent = state.dataset ? (p.message || 'No elements match these filters.') : 'Load the synthetic lab to begin.'; renderInspector(state.selected); renderPager(p); state.exportButton.disabled = !state.dataset; return; }
    const width = 1100; const hostRows = Math.ceil(nodes.filter((item) => typeOf(item) === 'host').length / 5); const height = state.view === 'l2' ? Math.max(690, 280 + Math.max(0, hostRows - 1) * 82 + 68 + 32) : 600; state.drawing.setAttribute('viewBox', `0 0 ${width} ${height}`);
    const positions = layoutNodes(nodes, width, state.view);
    edges.forEach((edge) => { const a = positions.get(str(first(edge.source_id, edge.source, edge.from, edge.source_host_id))); const b = positions.get(str(first(edge.destination_id, edge.target, edge.to, edge.destination_host_id))); if (!a || !b) return; const line = svg('path', { d: `M${a.x},${a.y} L${b.x},${b.y}`, class: `tl-edge ${edge.path ? 'tl-path-edge' : ''}`, 'data-link-id': first(edge.id, '') }); state.stage.append(line); });
    nodes.forEach((item) => { const point = positions.get(idOf(item)); const kind = typeOf(item); const group = svg('g', { class: `tl-node tl-kind-${kind}`, tabindex: '0', role: 'button', 'aria-label': `${kind}: ${labelOf(item)}`, transform: `translate(${point.x} ${point.y})` }); group.__item = item;
      const box = svg('rect', { x: '-80', y: '-34', width: '160', height: '68', rx: '12', class: 'tl-node-box' }); const title = svg('text', { x: '-42', y: '-2', class: 'tl-node-title' }); title.textContent = clipped(labelOf(item), 19); const sub = svg('text', { x: '-42', y: '17', class: 'tl-node-subtitle' }); sub.textContent = clipped(str(first(item.subtitle, item.cidr, item.ip, item.host_count && `${item.host_count} hosts`), kind), 23); group.append(box, makeIcon(kind, item), title, sub); state.stage.append(group);
    });
    state.caption.textContent = str(first(p.caption, p.message), `${shown} elements shown · configuration-derived map · live observations unavailable`);
    state.exportButton.disabled = !state.dataset;
    renderPager(p); renderHostOptions(nodes); renderInspector(state.selected); state.scale = 1; state.x = 0; state.y = 0; applyTransform(); renderPath(p.path);
  }
  function renderPager(p) { const total = Number(first(p.pagination && p.pagination.total, p.total, p.total_count, p.filtered_count, 0)); const pages = Math.max(1, Math.ceil(total / state.limit)); const page = Math.floor(state.offset / state.limit) + 1; state.pageLabel.textContent = `Hosts ${state.offset + 1}–${Math.min(state.offset + state.limit, total)} of ${total}`; state.prev.disabled = state.offset <= 0; state.next.disabled = p.pagination ? !p.pagination.has_more : state.offset + state.limit >= total; state.pager.hidden = state.view !== 'l2' || total <= state.limit; }
  function renderHostOptions(nodes) { state.options.replaceChildren(); const candidates = [...arr(state.dataset && state.dataset.hosts), ...nodes, ...arr(state.projection && state.projection.hosts)]; const seen = new Set(); candidates.filter((item) => typeOf(item) === 'host' && !seen.has(idOf(item)) && seen.add(idOf(item))).forEach((item) => { const option = el('option'); option.value = `${labelOf(item)} (${idOf(item)})`; option.dataset.id = idOf(item); option.dataset.host = labelOf(item); state.options.append(option); }); }
  function findHostId(value) { const match = [...state.options.options].find((o) => o.value === value || o.dataset.id === value || o.dataset.host === value); if (match) return match.dataset.id; const host = arr(state.dataset && state.dataset.hosts).find((item) => item.id === value || item.ip === value || item.mac === value); return host ? host.id : ''; }
  function selectItem(item) { state.selected = item; renderInspector(item); const kind = typeOf(item); if (kind === 'network' || kind === 'subnet') { state.network = idOf(item); state.net.value = state.network; state.zone = ''; state.zoneInput.value = ''; state.view = 'l2'; state.offset = 0; drawTabs(); requestProjection(); } else if (kind === 'zone') { state.zone = str(item.zone, idOf(item).replace(/^zone-/, '')); state.zoneInput.value = state.zone; state.network = ''; state.net.value = ''; state.view = 'l3'; state.offset = 0; drawTabs(); requestProjection(); } else if (kind === 'host') { state.source = idOf(item); state.sourceField.input.value = `${labelOf(item)} (${idOf(item)})`; clearPathDisplay(); } }
  function renderInspector(item) {
    state.inspector.replaceChildren();
    if (!item) { state.inspector.append(el('div', 'tl-empty', 'Select a network, zone, or host to inspect its details.')); return; }
    const kind = typeOf(item); state.inspector.append(el('span', 'tl-eyebrow', `${kind} detail`), el('h3', '', labelOf(item)));
    const badge = el('span', 'tl-source-badge', str(first(item.source_badge, item.source, 'Synthetic lab dataset'))); state.inspector.append(badge);
    const list = el('dl', 'tl-details'); const data = kind === 'host' ? [['Host ID', idOf(item)], ['IP address', first(item.ip, item.ip_address, item.address)], ['MAC address', first(item.mac, item.mac_address)], ['Network', first(item.network_id, item.network)], ['Zone', first(item.zone_id, item.zone)], ['VLAN', first(item.vlan, item.vlan_id)], ['Port', first(item.port, item.ports && portsText(item.ports))], ['Attachment', item.attachment]] : [['ID', idOf(item)], ['CIDR', first(item.cidr, item.subnet, item.prefix)], ['Zone', first(item.zone_id, item.zone)], ['Hosts', first(item.host_count, item.count)]];
    data.filter(([, value]) => value !== undefined && value !== null && value !== '').forEach(([k, v]) => { list.append(el('dt', '', k), el('dd', '', Array.isArray(v) ? v.join(', ') : String(v))); }); state.inspector.append(list);
    const evidence = arr(first(item.evidence, item.sources, item.source_lines)); if (evidence.length) { const btn = button('tl-evidence-button', `View ${evidence.length} source references`); btn.addEventListener('click', () => showSources(resolveSources(evidence))); state.inspector.append(btn); }
  }
  function portsText(ports) { return arr(ports).map((p) => typeof p === 'object' ? `${str(first(p.name, p.id, p.port))}${p.vlan ? ` · VLAN ${p.vlan}` : ''}${p.mac ? ` · ${p.mac}` : ''}` : String(p)).join(', '); }
  function resolveSources(refs) { const available = arr(state.projection && state.projection.sources); const grouped = new Map(); for (const ref of refs) { const source = available.find((item) => item.id === ref.source_id || item.id === ref.id); const key = source ? source.id : str(ref.source_id, str(ref.id, 'unresolved')); if (!grouped.has(key)) grouped.set(key, { source, refs: [] }); grouped.get(key).refs.push(ref); } return [...grouped.values()]; }
  function showSources(entries) { state.sourceBody.replaceChildren(); arr(entries).forEach(({ source, refs }) => { const card = el('article', 'tl-source-card'); card.append(el('strong', '', str(first(source && source.label, source && source.kind, 'Auxiliary evidence')))); const hash = source && first(source.sha256, source.hash); if (hash) card.append(el('code', 'tl-hash', `SHA-256 ${hash}`)); const lines = el('pre', ''); const wanted = new Set(arr(refs).map((ref) => Number(ref.line)).filter(Number.isFinite)); const excerpts = arr(source && source.lines).filter((line) => wanted.has(Number(line.number))); const refsLabel = arr(refs).map((ref) => str(ref.ref, `line ${ref.line}`)).join(', '); const body = excerpts.map((line) => `${line.number}: ${str(line.text)}`).join('\n'); lines.textContent = body || 'Referenced source lines are unavailable.'; if (refsLabel) card.append(el('span', 'tl-source-ref', refsLabel)); card.append(lines); state.sourceBody.append(card); }); document.getElementById('tl-sources').open = true; }
  function renderPath(path) {
    state.traceStatus.replaceChildren();
    if (!path) return;
    state.traceStatus.append(el('strong', '', str(first(path.disposition, path.status), 'Path evaluated')));
    const hops = arr(first(path.hops, path.nodes, path.path));
    if (hops.length) {
      const list = el('ol', 'tl-hop-list');
      hops.forEach((hop) => {
        const entry = el('li');
        entry.append(el('strong', '', str(first(hop.label, hop.name, hop.node_id, hop.id))));
        entry.append(el('span', 'tl-hop-ports', `${str(hop.ingress, 'Origin')} → ${str(hop.egress, 'Destination')} · VLAN ${str(hop.vlan, 'unavailable')}`));
        if (arr(hop.evidence).length) {
          const source = button('', 'Source', 'tl-button tl-hop-source');
          source.setAttribute('aria-label', `Source evidence for ${str(hop.node_id)}`);
          source.addEventListener('click', () => showSources(resolveSources(hop.evidence)));
          entry.append(source);
        }
        list.append(entry);
      });
      state.traceStatus.append(list);
    }
    if (path.status === 'requires_l3') state.traceStatus.append(el('span', 'tl-callout', 'These hosts are on different networks or VLANs; an L2 path cannot be established.'));
    else if (path.status === 'indeterminate') state.traceStatus.append(el('span', 'tl-callout', 'The available link evidence does not establish a path.'));
    state.traceStatus.append(el('span', 'tl-callout', 'Runtime observations unavailable; this result uses the synthetic lab configuration.'));
    const pathIds = new Set(hops.map((hop) => str(first(hop.host_id, hop.id, hop.node_id, hop)))); state.stage.querySelectorAll('.tl-node').forEach((node) => { if (pathIds.has(idOf(node.__item))) node.classList.add('tl-on-path'); });
    const linkIds = new Set(arr(path.links).map(String)); state.stage.querySelectorAll('.tl-edge').forEach((edge) => { if (linkIds.has(edge.dataset.linkId)) edge.classList.add('tl-path-edge'); });
  }
  async function requestPath() {
    const s = findHostId(state.sourceField.input.value); const d = findHostId(state.destinationField.input.value);
    if (!s || !d || s === d) { clearPathDisplay(); state.traceStatus.textContent = 'Choose two distinct lab hosts by name or ID.'; return; }
    state.source = s; state.destination = d;
    const sourceHost = arr(state.dataset && state.dataset.hosts).find((host) => host.id === s);
    if (sourceHost) { state.view = 'l2'; state.network = sourceHost.network_id; state.net.value = state.network; state.zone = ''; state.zoneInput.value = ''; state.offset = 0; drawTabs(); }
    const projection = await requestProjection();
    if (!projection) return;
    if (!projection.path) state.traceStatus.textContent = 'The current projection did not include a path result.';
  }
  function fitGraph() { state.scale = 1; state.x = 0; state.y = 0; applyTransform(); }
  function setZoom(value) { state.scale = Math.max(.35, Math.min(3, value)); applyTransform(); }
  function applyTransform() { if (!state.stage) return; state.stage.setAttribute('transform', `translate(${state.x} ${state.y}) scale(${state.scale})`); state.zoomLabel.textContent = `${Math.round(state.scale * 100)}%`; }
  function toggleFullscreen() { state.fullscreenActive = !state.fullscreenActive; state.root.classList.toggle('tl-fullscreen', state.fullscreenActive); state.fullscreenButton.setAttribute('aria-pressed', String(state.fullscreenActive)); state.fullscreenButton.textContent = state.fullscreenActive ? 'Exit full screen' : 'Full screen'; if (state.fullscreenActive) { state.fullscreenFocus = document.activeElement; state.bodyOverflow = document.body.style.overflow; document.body.style.overflow = 'hidden'; state.graph.focus(); } else { document.body.style.overflow = state.bodyOverflow || ''; if (state.fullscreenFocus && state.fullscreenFocus.isConnected) state.fullscreenFocus.focus(); state.fullscreenFocus = null; } }
  function exportDataset() { if (!state.dataset) return; const blob = new Blob([JSON.stringify(state.dataset, null, 2)], { type: 'application/json' }); const url = URL.createObjectURL(blob); const link = el('a'); link.href = url; link.download = 'graphlayer-synthetic-topology-lab.json'; link.click(); setTimeout(() => URL.revokeObjectURL(url), 0); }
  function idOf(item) { return str(first(item.id, item.node_id, item.host_id, item.network_id, item.zone_id, item.subnet_id)); }
  function labelOf(item) { return str(first(item.label, item.name, item.hostname, item.display_name, item.cidr, idOf(item)), 'Network element'); }
  function typeOf(item) { const raw = str(first(item.role, item.type, item.kind, item.element_type, item.node_type), 'host').toLowerCase(); if (/zone/.test(raw)) return 'zone'; if (/network|subnet|segment|vlan/.test(raw)) return raw.includes('subnet') ? 'subnet' : 'network'; if (/firewall|router|switch|gateway/.test(raw)) return 'device'; return 'host'; }
  function makeIcon(kind, item = {}) { const icon = svg('g', { class: 'tl-icon', transform: 'translate(-63 -8)', 'aria-hidden': 'true' });
    if (kind === 'host') { icon.append(svg('rect', { x: '-8', y: '-7', width: '16', height: '12', rx: '2' }), svg('path', { d: 'M-4 9h8M0 5v4' })); }
    else if (kind === 'network' || kind === 'subnet') { icon.append(svg('path', { d: 'M-7 6 0-5 7 6M-7 6H7' }), svg('circle', { cx: '0', cy: '-6', r: '2.5' }), svg('circle', { cx: '-7', cy: '7', r: '2.5' }), svg('circle', { cx: '7', cy: '7', r: '2.5' })); }
    else if (kind === 'zone') { icon.append(svg('path', { d: 'M0-10 9-6v6c0 5-4 8-9 11-5-3-9-6-9-11v-6z' }), svg('path', { d: 'M-4 0 0-4 4 0 0 4z' })); }
    else if (/firewall/.test(str(item.role, '').toLowerCase())) { icon.append(svg('path', { d: 'M0-10 9-6v6c0 5-4 8-9 11-5-3-9-6-9-11v-6z' }), svg('path', { d: 'M-5-3h4v3h-4zM1-3h4v3H1zM-5 3h4v3h-4zM1 3h4v3H1z' })); }
    else if (/router/.test(str(item.role, '').toLowerCase())) { icon.append(svg('circle', { cx: '0', cy: '0', r: '9' }), svg('path', { d: 'M-5-2h10l-2-2m2 2-2 2M5 3H-5l2-2m-2 2 2 2' })); }
    else if (/switch/.test(str(item.role, '').toLowerCase())) { icon.append(svg('rect', { x: '-10', y: '-7', width: '20', height: '14', rx: '3' }), svg('path', { d: 'M-6-2h2M-1-2h2M4-2h2M-6 3h2M-1 3h2M4 3h2' })); }
    else { icon.append(svg('rect', { x: '-9', y: '-9', width: '18', height: '18', rx: '3' }), svg('path', { d: 'M-5-4h10M-5 0h10M-5 4h10' })); }
    return icon;
  }
  function clipped(value, n) { const s = String(value); return s.length > n ? `${s.slice(0, n - 1)}…` : s; }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', setup, { once: true }); else setup();
  window.GraphLayerLayers = { setSession, reload: loadDataset, refresh: requestProjection };
})();
