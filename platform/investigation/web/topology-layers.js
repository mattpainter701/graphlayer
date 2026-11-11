(() => {
  'use strict';

  const state = { root: null, csrf: '', dataset: null, projection: null, view: 'l3', network: '', zone: '', vrf: '', vdom: '', search: '', offset: 0, limit: 25, source: '', sourcePinned: false, destination: '', busy: false, seq: 0, timer: 0, controller: null, scale: 1, x: 0, y: 0, pan: null, selected: null, preservePan: false, transformFrame: 0, responsivePathFrame: 0, renderedPathOrientation: '', inspectorCollapsed: true, fullscreenActive: false, fullscreenFocus: null, bodyOverflow: null, focusHostId: '', focusNotice: '', focusCameraTarget: '', backStack: [] };
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
    const vrf = field('VRF', 'tl-vrf'); vrf.input.placeholder = 'All VRFs';
    const vdom = field('VDOM', 'tl-vdom'); vdom.input.placeholder = 'All VDOMs';
    const networkOptions = el('datalist'); networkOptions.id = 'tl-network-options'; net.input.setAttribute('list', networkOptions.id);
    const zoneOptions = el('datalist'); zoneOptions.id = 'tl-zone-options'; zone.input.setAttribute('list', zoneOptions.id);
    const vrfOptions = el('datalist'); vrfOptions.id = 'tl-vrf-options'; vrf.input.setAttribute('list', vrfOptions.id);
    const vdomOptions = el('datalist'); vdomOptions.id = 'tl-vdom-options'; vdom.input.setAttribute('list', vdomOptions.id);
    const search = field('SEARCH', 'tl-search', 'search'); search.input.placeholder = 'Host, IP, MAC…'; search.input.setAttribute('aria-label', 'Search hosts and networks');
    const resetFilters = button('tl-reset-filters', 'Reset filters'); resetFilters.classList.add('tl-reset-filters');
    const applyFilters = button('tl-apply-filters', 'Apply filters', 'tl-button tl-apply-filters');
    controls.append(net.wrap, zone.wrap, vrf.wrap, vdom.wrap, search.wrap, resetFilters, applyFilters);
    const activeFilters = el('div', 'tl-active-filters'); activeFilters.id = 'tl-active-filters'; activeFilters.setAttribute('aria-label', 'Active filters');
    const filterCount = el('span', 'tl-filter-count', 'No active filters'); filterCount.id = 'tl-filter-count'; filterCount.setAttribute('aria-live', 'polite');
    const canvasHead = el('div', 'tl-canvas-head');
    const graphTitle = el('strong', '', 'Layer 3 overview');
    const actions = el('div', 'tl-actions');
    const focusPath = button('tl-focus-path', 'Show all attachments'); focusPath.hidden = true;
    const fit = button('tl-fit', 'Fit'); const focusSelection = button('tl-focus-selection', 'Focus selection'); focusSelection.disabled = true;
    const exportView = button('tl-export-view', 'Export SVG'); exportView.disabled = true; exportView.title = 'Export the current rendered map as an SVG image.'; exportView.setAttribute('aria-label', 'Export the current network map as SVG');
    const zoomOut = button('tl-zoom-out', '−', 'tl-button tl-square'); const zoomLabel = el('output', 'tl-zoom', '100%'); zoomLabel.id = 'tl-zoom'; const zoomIn = button('tl-zoom-in', '+', 'tl-button tl-square'); const fullscreen = button('tl-fullscreen', 'Full screen'); fullscreen.setAttribute('aria-pressed', 'false');
    const inspectorToggle = button('tl-inspector-toggle', 'Details'); inspectorToggle.setAttribute('aria-controls', 'tl-inspector'); inspectorToggle.setAttribute('aria-expanded', 'false'); inspectorToggle.disabled = true;
    actions.append(focusPath, fit, focusSelection, exportView, zoomOut, zoomLabel, zoomIn, inspectorToggle, fullscreen); canvasHead.append(graphTitle, actions);
    const graph = el('div', 'tl-graph'); graph.id = 'tl-graph'; graph.tabIndex = 0; graph.setAttribute('role', 'tabpanel'); graph.setAttribute('aria-labelledby', 'tl-tab-l3'); graph.setAttribute('aria-label', 'Interactive network map');
    const drawing = svg('svg', { id: 'tl-svg', role: 'group', 'aria-label': 'Network topology diagram' }); const stage = svg('g', { id: 'tl-stage' }); drawing.append(stage); graph.append(drawing);
    const minimap = svg('svg', { id: 'tl-minimap', role: 'group', 'aria-label': 'Map overview', hidden: '' });
    const miniTitle = svg('title'); miniTitle.textContent = 'Map overview: click a node to focus it, or empty space to recenter. Focus the viewport and use arrow keys to move; Enter or Home fits the map.';
    const miniContent = svg('g', { 'aria-hidden': 'true', class: 'tl-minimap-content' });
    const miniViewport = svg('rect', { id: 'tl-minimap-viewport', tabindex: '0', role: 'button', 'aria-label': 'Map viewport. Arrow keys move the map; Enter fits the projection.' });
    minimap.append(miniTitle, miniContent, miniViewport); graph.append(minimap);
    const legend = el('div', 'tl-legend'); legend.setAttribute('aria-label', 'Network map legend');
    for (const [kind, role, label] of [['host', '', 'Host'], ['network', '', 'Network'], ['device', 'router', 'Router'], ['device', 'switch', 'Switch'], ['device', 'firewall', 'Firewall'], ['zone', '', 'Zone']]) { const item = el('span', 'tl-legend-item'); const iconSvg = svg('svg', { viewBox: '-12 -12 24 24', 'aria-hidden': 'true' }); const icon = makeIcon(kind, { role }); icon.setAttribute('transform', ''); iconSvg.append(icon); item.append(iconSvg, el('span', '', label)); legend.append(item); }
    const caption = el('div', 'tl-caption'); caption.id = 'tl-caption'; caption.setAttribute('role', 'status');
    const breadcrumbs = el('nav', 'tl-breadcrumbs'); breadcrumbs.id = 'tl-breadcrumbs'; breadcrumbs.setAttribute('aria-label', 'Map navigation');
    const back = button('tl-navigation-back', '← Back'); back.disabled = true; breadcrumbs.append(back);
    const pager = el('div', 'tl-pager'); pager.id = 'tl-pager';
    const firstPage = button('tl-first', 'First'); const prev = button('tl-prev', '← Previous'); const pageLabel = el('span', ''); pageLabel.id = 'tl-page-label'; const pagePosition = el('span', 'tl-page-position'); pagePosition.id = 'tl-page-position'; const next = button('tl-next', 'Next →'); const lastPage = button('tl-last', 'Last');
    const pageSizeLabel = el('label', 'tl-page-size', 'Rows'); const pageSize = el('select'); pageSize.id = 'tl-page-size'; pageSize.setAttribute('aria-label', 'Hosts per page');
    for (const size of [25, 50, 100]) { const option = el('option', '', size); option.value = String(size); pageSize.append(option); }
    pageSize.value = String(state.limit); pageSizeLabel.append(pageSize); pager.append(firstPage, prev, pageLabel, pagePosition, next, lastPage, pageSizeLabel);
    const hostResults = el('details', 'tl-host-results'); hostResults.id = 'tl-host-results'; hostResults.open = true;
    const hostResultsSummary = el('summary', '', 'Host results'); hostResultsSummary.id = 'tl-host-results-heading';
    const hostResultsCount = el('p', 'tl-host-results-count', 'Load the lab to browse matching hosts.'); hostResultsCount.id = 'tl-host-results-summary'; hostResultsCount.setAttribute('aria-live', 'polite');
    const hostResultsList = el('div', 'tl-host-results-list'); hostResultsList.setAttribute('role', 'list'); hostResults.append(hostResultsSummary, hostResultsCount, hostResultsList);
    const lower = el('div', 'tl-lower');
    const trace = el('section', 'tl-trace'); trace.setAttribute('aria-labelledby', 'tl-trace-title'); const traceTitle = el('h3', '', 'Trace a host path'); traceTitle.id = 'tl-trace-title'; trace.append(traceTitle);
    const traceCopy = el('p', 'tl-muted', 'Choose two hosts from the current projection to trace the configured Layer 2 path.'); trace.append(traceCopy);
    const source = field('SOURCE HOST', 'tl-source'); source.input.placeholder = 'Select a lab host'; source.input.setAttribute('list', 'tl-host-options');
    const destination = field('DESTINATION HOST', 'tl-destination'); destination.input.placeholder = 'Select a lab host'; destination.input.setAttribute('list', 'tl-host-options');
    const options = el('datalist'); options.id = 'tl-host-options'; const traceButton = button('tl-trace-button', 'Trace L2 path', 'tl-button tl-primary');
    const traceRow = el('div', 'tl-trace-row'); traceRow.append(source.wrap, destination.wrap, traceButton); trace.append(traceRow, options);
    const traceStatus = el('div', 'tl-trace-status'); traceStatus.id = 'tl-trace-status'; traceStatus.setAttribute('aria-live', 'polite'); trace.append(traceStatus);
    const inspector = el('aside', 'tl-inspector'); inspector.id = 'tl-inspector'; inspector.setAttribute('aria-label', 'Selection details'); inspector.setAttribute('aria-hidden', 'true'); inspector.append(el('div', 'tl-empty', 'Select a network, zone, or host to inspect its details.'));
    lower.append(trace);
    const sourceDrawer = el('details', 'tl-sources'); sourceDrawer.id = 'tl-sources'; const sourceSummary = el('summary', '', 'Auxiliary source evidence'); const sourceBody = el('div', 'tl-source-body'); sourceBody.id = 'tl-source-body'; sourceDrawer.append(sourceSummary, sourceBody);
    const note = el('p', 'tl-note', 'All lab devices and observations are synthetic. Configuration projections do not establish live reachability or runtime state.');
    const workspace = el('div', 'tl-workspace'); workspace.append(canvasHead, graph, legend, caption, breadcrumbs, pager, hostResults);
    const content = el('div', 'tl-content'); content.append(workspace, inspector); content.classList.add('is-inspector-collapsed');
    state.root.replaceChildren(header, summary, tabs, controls, filterCount, activeFilters, networkOptions, zoneOptions, vrfOptions, vdomOptions, content, lower, sourceDrawer, note);
    Object.assign(state, { load, exportButton, exportView, summary, tabs, net: net.input, zoneInput: zone.input, vrfInput: vrf.input, vdomInput: vdom.input, searchInput: search.input, resetFilters, activeFilters, filterCount, networkOptions, zoneOptions, vrfOptions, vdomOptions, pageSize, graphTitle, drawing, stage, graph, legend, caption, pager, prev, next, pageLabel, firstPage, lastPage, pagePosition, breadcrumbs, back, hostResults, hostResultsCount, hostResultsList, content, inspector, inspectorToggle, sourceField: source, destinationField: destination, options, traceButton, traceStatus, sourceBody, fullscreenButton: fullscreen, zoomLabel, applyFilters });
    Object.assign(state, { focusSelection, minimap, miniContent, miniViewport, rovingKey: '', focusCameraKey: '' });
    state.focusPath = true; state.focusToggle = focusPath;
    focusPath.addEventListener('click', () => { state.focusPath = !state.focusPath; render(); });
    load.addEventListener('click', loadDataset);
    exportButton.addEventListener('click', exportDataset);
    exportView.addEventListener('click', exportCurrentView);
    tabs.addEventListener('click', (e) => { const b = e.target.closest('[data-view]'); if (b) chooseView(b.dataset.view); });
    tabs.addEventListener('keydown', (e) => { const active = e.target.closest('[data-view]'); if (!active) return; const buttons = [...tabs.querySelectorAll('[data-view]')]; let i = buttons.indexOf(active); if (e.key === 'ArrowRight') i = (i + 1) % buttons.length; else if (e.key === 'ArrowLeft') i = (i - 1 + buttons.length) % buttons.length; else if (e.key === 'Home') i = 0; else if (e.key === 'End') i = buttons.length - 1; else return; e.preventDefault(); chooseView(buttons[i].dataset.view); buttons[i].focus(); });
    const filterInputs = [net.input, zone.input, vrf.input, vdom.input, search.input];
    filterInputs.forEach((input) => {
      input.addEventListener('input', scheduleFilterApply);
      input.addEventListener('keydown', (event) => { if (event.key === 'Enter') { event.preventDefault(); applyFilterInputs(); } });
    });
    applyFilters.addEventListener('click', applyFilterInputs);
    resetFilters.addEventListener('click', resetAllFilters);
    activeFilters.addEventListener('click', (e) => { const b = e.target.closest('[data-filter-kind]'); if (b) clearFilter(b.dataset.filterKind); });
    pageSize.addEventListener('change', () => { const nextLimit = Number(pageSize.value); if (![25, 50, 100].includes(nextLimit)) return; state.limit = nextLimit; state.offset = 0; requestProjection(); });
    firstPage.addEventListener('click', () => navigatePage(0)); lastPage.addEventListener('click', () => navigatePage(Number(state.projection?.navigation?.last_offset || 0)));
    prev.addEventListener('click', () => { const value = state.projection?.navigation?.previous_offset; if (Number.isFinite(value)) navigatePage(value); });
    next.addEventListener('click', () => { const value = state.projection?.navigation?.next_offset; if (Number.isFinite(value)) navigatePage(value); });
    back.addEventListener('click', navigateBack);
    hostResultsList.addEventListener('click', (event) => { const action = event.target.closest('button[data-action]'); const row = event.target.closest('[data-host-id]'); if (!action || !row) return; const host = arr(state.projection?.hosts).find((item) => idOf(item) === row.dataset.hostId); if (!host) return; if (action.dataset.action === 'inspect') inspectHost(host); else if (action.dataset.action === 'map') focusHostOnMap(host); else if (action.dataset.action === 'source') setTraceEndpoint('source', host); else if (action.dataset.action === 'destination') setTraceEndpoint('destination', host); });
    fit.addEventListener('click', fitGraph); zoomIn.addEventListener('click', () => setZoom(state.scale * 1.2)); zoomOut.addEventListener('click', () => setZoom(state.scale / 1.2));
    focusSelection.addEventListener('click', () => focusGraphItem(selectedGraphItem()));
    minimap.addEventListener('pointerdown', (event) => event.stopPropagation());
    minimap.addEventListener('click', (event) => {
      const target = event.target.closest('[data-mini-key]');
      if (target) {
        event.preventDefault(); event.stopPropagation();
        const item = graphItems().find((candidate) => graphItemKey(candidate) === target.dataset.miniKey);
        if (item) focusAndCenterGraphItem(item);
        return;
      }
      recenterFromOverview(event);
    });
    minimap.addEventListener('keydown', navigateOverview);
    drawing.addEventListener('focusin', (event) => { const item = event.target.closest('.tl-node,.tl-edge'); if (item) syncGraphRover(item); });
    drawing.addEventListener('keydown', navigateGraphItems);
    fullscreen.addEventListener('click', toggleFullscreen);
    inspectorToggle.addEventListener('click', () => { if (!state.selected) return; state.inspectorCollapsed = !state.inspectorCollapsed; syncInspectorView(); });
    graph.addEventListener('wheel', (e) => { e.preventDefault(); const point = screenPoint(e); zoomAt(state.scale * (e.deltaY < 0 ? 1.12 : 1 / 1.12), point.x, point.y); }, { passive: false });
    graph.addEventListener('pointerdown', (e) => { if (e.button !== 0 || e.target.closest('.tl-node,.tl-edge,#tl-minimap')) return; state.focusCameraKey = ''; const point = screenPoint(e); state.pan = { x: point.x, y: point.y, ox: state.x, oy: state.y }; graph.setPointerCapture(e.pointerId); graph.classList.add('is-panning'); });
    graph.addEventListener('pointermove', (e) => { if (!state.pan) return; const point = screenPoint(e); state.x = state.pan.ox + point.x - state.pan.x; state.y = state.pan.oy + point.y - state.pan.y; scheduleTransform(); });
    const stopPan = (e) => { if (e && graph.hasPointerCapture(e.pointerId)) graph.releasePointerCapture(e.pointerId); state.pan = null; graph.classList.remove('is-panning'); }; graph.addEventListener('pointerup', stopPan); graph.addEventListener('pointercancel', stopPan); graph.addEventListener('lostpointercapture', stopPan);
    graph.addEventListener('keydown', (e) => { if (e.target.closest('.tl-node') || e.target.matches('button, input, select')) return; const step = e.shiftKey ? 80 : 32; if (e.key === '+' || e.key === '=') setZoom(state.scale * 1.2); else if (e.key === '-' || e.key === '_') setZoom(state.scale / 1.2); else if (e.key === 'Home') fitGraph(); else if (e.key === 'ArrowLeft') state.x += step; else if (e.key === 'ArrowRight') state.x -= step; else if (e.key === 'ArrowUp') state.y += step; else if (e.key === 'ArrowDown') state.y -= step; else return; e.preventDefault(); state.focusCameraKey = ''; scheduleTransform(); });
    drawing.addEventListener('click', (e) => { const item = e.target.closest('.tl-node'); if (item) selectItem(item.__item); });
    drawing.addEventListener('keydown', (e) => { if (!['Enter', ' '].includes(e.key)) return; const item = e.target.closest('.tl-node'); if (item) { e.preventDefault(); selectItem(item.__item); } });
    traceButton.addEventListener('click', requestPath);
    source.input.addEventListener('input', () => { state.source = ''; state.sourcePinned = true; clearPathDisplay(); }); destination.input.addEventListener('input', () => { state.destination = ''; clearPathDisplay(); });
    source.input.addEventListener('change', () => { state.source = findHostId(source.input.value); }); destination.input.addEventListener('change', () => { state.destination = findHostId(destination.input.value); });
    window.addEventListener('resize', scheduleResponsivePathLayout);
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && state.fullscreenActive) toggleFullscreen(); });
    window.addEventListener('graphlayer:session', (e) => setSession(e.detail && e.detail.csrf));
    drawTabs(); syncInspectorView(); renderFilters(); state.caption.textContent = 'Load the synthetic lab to explore this map.'; state.pager.hidden = true; updateControls();
  }

  function setSession(csrf) { const next = str(csrf); if (next !== state.csrf) { clearTimeout(state.timer); state.timer = 0; if (state.controller) { state.seq += 1; state.controller.abort(); state.controller = null; } state.busy = false; state.focusHostId = ''; state.focusNotice = ''; setProjectionBusy(false); } state.csrf = next; if (!state.csrf && state.stage) clearProjectionUI('Waiting for the local session…'); updateControls(); }
  function updateControls() { if (state.applyFilters) state.applyFilters.disabled = !hasPendingFilters() || !state.csrf || !state.dataset; if (state.load) state.load.disabled = !state.csrf || state.busy; if (state.traceButton) state.traceButton.disabled = !state.csrf || state.busy || !state.dataset; if (state.exportView) state.exportView.disabled = state.busy || !state.projection || typeof window.GraphLayerGraphExport?.download !== 'function'; updateViewportControls(); }
  function setProjectionBusy(busy) {
    if (!state.inspector) return;
    state.inspector.setAttribute('aria-busy', String(Boolean(busy)));
    state.inspector.inert = Boolean(busy);
    updateViewportControls();
  }
  function filterDraft() { return { network: state.net.value.trim(), zone: state.zoneInput.value.trim(), vrf: state.vrfInput.value.trim(), vdom: state.vdomInput.value.trim(), search: state.searchInput.value.trim() }; }
  function hasPendingFilters() { const draft = filterDraft(); return draft.network !== state.network || draft.zone !== state.zone || draft.vrf !== state.vrf || draft.vdom !== state.vdom || draft.search !== state.search; }
  function scheduleFilterApply() {
    clearTimeout(state.timer); state.timer = 0;
    const pending = hasPendingFilters();
    const draft = pending ? filterDraft() : null;
    if (pending) state.filterCount.textContent = `Filter edits pending · ${Object.values(draft).filter(Boolean).length} active filters`; else renderFilters();
    updateControls();
    if (pending && state.csrf && state.dataset) state.timer = setTimeout(applyFilterInputs, 220);
  }
  function applyFilterInputs() {
    clearTimeout(state.timer); state.timer = 0;
    if (!hasPendingFilters()) { renderFilters(); updateControls(); return; }
    const draft = filterDraft();
    Object.assign(state, draft); state.offset = 0;
    clearScopeSelection();
    renderFilters(); updateControls();
    // Filters are local view state and can be edited before a lab/session is
    // ready. Projection requests remain guarded until both are available.
    if (state.csrf && state.dataset) requestProjection();
  }
  function renderFilters() {
    if (!state.activeFilters) return;
    const filters = [['network', state.network], ['zone', state.zone], ['vrf', state.vrf], ['vdom', state.vdom], ['search', state.search]].filter(([, value]) => value);
    const pending = hasPendingFilters();
    const draft = pending ? filterDraft() : null;
    const draftCount = pending ? Object.values(draft).filter(Boolean).length : filters.length;
    state.filterCount.textContent = pending ? `Filter edits pending · ${draftCount} active filters` : (filters.length ? `${filters.length} active filter${filters.length === 1 ? '' : 's'}` : 'No active filters');
    const activeKinds = new Set();
    const chips = new Map([...state.activeFilters.querySelectorAll('[data-filter-kind]')].map((chip) => [chip.dataset.filterKind, chip]));
    filters.forEach(([kind, value]) => {
      activeKinds.add(kind);
      let chip = chips.get(kind);
      if (!chip) { chip = button('', '', 'tl-filter-chip'); chip.dataset.filterKind = kind; }
      const title = ({ search: 'Search', network: 'Network', zone: 'Zone', vrf: 'VRF', vdom: 'VDOM' })[kind] || kind;
      chip.replaceChildren(el('span', '', `${title}: ${value}`), el('span', 'tl-filter-remove', '×'));
      chip.setAttribute('aria-label', `Remove ${kind} filter ${value}`);
      if (chip.parentElement !== state.activeFilters) state.activeFilters.append(chip);
    });
    chips.forEach((chip, kind) => { if (!activeKinds.has(kind)) chip.remove(); });
    const facets = state.projection && state.projection.facets || {};
    state.networkOptions.replaceChildren(); arr(facets.networks).forEach((item) => { const option = el('option'); option.value = str(item.value); option.label = `${str(item.label, item.value)} · ${Number(item.count || 0)}`; if (option.value) state.networkOptions.append(option); });
    state.zoneOptions.replaceChildren(); arr(facets.zones).forEach((item) => { const option = el('option'); option.value = str(item.value); option.label = `${str(item.label, item.value)} · ${Number(item.count || 0)}`; if (option.value) state.zoneOptions.append(option); });
    state.vrfOptions.replaceChildren(); arr(facets.vrfs).forEach((value) => { const option = el('option'); option.value = String(value); state.vrfOptions.append(option); });
    state.vdomOptions.replaceChildren(); arr(facets.vdoms).forEach((value) => { const option = el('option'); option.value = String(value); state.vdomOptions.append(option); });
  }
  function clearFilter(kind) {
    const input = ({ network: state.net, zone: state.zoneInput, search: state.searchInput, vrf: state.vrfInput, vdom: state.vdomInput })[kind];
    if (!input) return;
    input.value = '';
    // Apply the remaining draft values too. Otherwise removing a chip cancels
    // the debounce while leaving other edited filters stranded in the inputs.
    applyFilterInputs();
    // renderFilters removes the activated chip. Return keyboard and pointer
    // users to the corresponding field rather than letting focus fall to body.
    input.focus({ preventScroll: true });
  }
  function resetAllFilters() {
    state.network = ''; state.zone = ''; state.vrf = ''; state.vdom = ''; state.search = ''; state.offset = 0; clearTimeout(state.timer); state.timer = 0; clearScopeSelection();
    state.net.value = ''; state.zoneInput.value = ''; state.vrfInput.value = ''; state.vdomInput.value = ''; state.searchInput.value = ''; renderFilters(); requestProjection();
  }
  function clearScopeSelection() {
    state.selected = null; state.selectedEdge = null; state.inspectorCollapsed = true; syncInspectorView(); renderInspector(null); refreshNodeSelection(); refreshEdgeSelection();
    state.traceRequested = false; state.focusPath = false; state.focusToggle.hidden = true; state.traceStatus.replaceChildren();
    state.stage.querySelectorAll('.tl-node.tl-on-path').forEach((node) => node.classList.remove('tl-on-path'));
    state.stage.querySelectorAll('.tl-edge.tl-path-edge').forEach((edge) => edge.classList.remove('tl-path-edge'));
    refreshEdgeSelection();
    if (!state.sourcePinned) { state.source = ''; state.sourceField.input.value = ''; }
    state.destination = ''; state.destinationField.input.value = '';
  }
  async function post(url, body, signal) {
    const response = await fetch(url, { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': state.csrf }, body: JSON.stringify(body), signal });
    let payload; try { payload = await response.json(); } catch (_) { throw new Error(`Request failed (${response.status})`); }
    if (!response.ok) { const error = payload && payload.error; throw new Error(str(error && (error.message || error.code), `Request failed (${response.status})`)); }
    return payload;
  }
  async function loadDataset() {
    if (!state.csrf || state.busy) return;
    clearTimeout(state.timer); state.timer = 0;
    const seq = ++state.seq; state.controller?.abort(); const controller = new AbortController(); state.controller = controller;
    state.busy = true; state.dataset = null; state.projection = null; state.view = 'l3'; state.network = ''; state.zone = ''; state.vrf = ''; state.vdom = ''; state.search = ''; state.offset = 0; state.focusHostId = ''; state.backStack = []; state.source = ''; state.sourcePinned = false; state.destination = ''; state.selected = null; state.selectedEdge = null; state.net.value = ''; state.zoneInput.value = ''; state.vrfInput.value = ''; state.vdomInput.value = ''; state.searchInput.value = ''; state.sourceField.input.value = ''; state.destinationField.input.value = ''; renderFilters(); drawTabs(); clearProjectionUI('Generating synthetic lab dataset…'); setProjectionBusy(true); state.load.textContent = 'Loading lab…'; updateControls();
    try { const result = await post('/api/sample/topology-layers', {}, controller.signal); if (seq !== state.seq) return; const dataset = result.dataset || result; validateDataset(dataset); state.dataset = dataset; state.offset = 0; state.load.textContent = 'Reload lab dataset'; await requestProjection(); }
    catch (error) { if (seq === state.seq && error.name !== 'AbortError') showError(error); }
    finally { if (seq === state.seq) { state.busy = false; setProjectionBusy(false); state.load.textContent = state.dataset ? 'Reload lab dataset' : 'Load 1,000-host lab'; updateControls(); } }
  }
  function query() { return { dataset: state.dataset, view: state.view, network_id: state.network, zone: state.zone, vrf: state.vrf, vdom: state.vdom, search: state.search, offset: state.offset, limit: state.limit, focus_host_id: state.focusHostId, source_host_id: state.traceRequested ? state.source : '', destination_host_id: state.traceRequested ? state.destination : '' }; }
  async function requestProjection(resetPath = true) {
    if (hasPendingFilters()) { applyFilterInputs(); return; }
    if (!state.csrf || !state.dataset) return;
    clearTimeout(state.timer); state.timer = 0;
    if (resetPath) { state.traceRequested = false; state.focusPath = false; }
    const seq = ++state.seq; state.controller?.abort(); const controller = new AbortController(); state.controller = controller;
    const request = query(); state.focusHostId = ''; state.busy = true; state.projection = null; renderFilters(); clearProjectionUI('Updating network map…'); state.inspector.replaceChildren(el('div', 'tl-empty', 'Updating selection details…')); setProjectionBusy(true); updateControls();
    try {
      const result = await post('/api/topology-layers', request, controller.signal);
      if (seq !== state.seq) return null;
      if (request.source_host_id && (!state.traceRequested || request.source_host_id !== state.source || request.destination_host_id !== state.destination)) return null;
      const projection = result.projection || result; validateProjection(projection); state.projection = projection; state.offset = Number(projection.pagination.offset ?? state.offset); state.focusNotice = ''; state.focusCameraTarget = ''; const focusHostId = str(request.focus_host_id); if (focusHostId && projection.navigation?.focus_host_id === focusHostId && projection.navigation?.focus_status !== 'visible') state.focusNotice = projection.navigation.focus_status === 'outside_filters' ? 'The selected host is outside the current filters. Adjust the filters to include it.' : 'The selected host is not available in this lab projection.'; else if (focusHostId && projection.navigation?.focus_status === 'visible') { const host = arr(projection.hosts).find((item) => idOf(item) === focusHostId); if (host && projectionNodes().some((item) => idOf(item) === focusHostId)) { state.selected = host; state.inspectorCollapsed = false; state.focusCameraTarget = focusHostId; } } renderFilters(); render(); return projection;
    }
    catch (error) { if (seq === state.seq && error.name !== 'AbortError') showError(error); return null; }
    finally { if (seq === state.seq) { state.busy = false; state.controller = null; setProjectionBusy(false); updateControls(); if (state.projection) { renderPager(state.projection); renderBreadcrumbs(); } focusPendingHost(); } }
  }
  function validateDataset(dataset) { if (!isObject(dataset) || dataset.schema_version !== 1 || dataset.synthetic !== true || !Array.isArray(dataset.hosts) || dataset.hosts.length > 10000 || !Array.isArray(dataset.networks) || !Array.isArray(dataset.sources) || dataset.sources.length > 100 || dataset.hosts.some((host) => !isObject(host) || !host.id) || dataset.sources.some((source) => !isObject(source) || !source.id || !Array.isArray(source.lines) || !source.sha256)) throw new Error('The lab response has an unsupported dataset shape.'); }
  function validateProjection(projection) { if (!isObject(projection) || projection.schema_version !== 1 || projection.synthetic !== true || !Array.isArray(projection.nodes) || projection.nodes.length > 140 || !Array.isArray(projection.links) || projection.links.length > 5000 || !Array.isArray(projection.sources) || !isObject(projection.counts) || !isObject(projection.pagination) || projection.nodes.some((item) => !isObject(item) || !idOf(item)) || projection.links.some((edge) => !isObject(edge) || !first(edge.id, edge.from, edge.source_id) || !first(edge.to, edge.target, edge.destination_id)) || (projection.hosts !== undefined && (!Array.isArray(projection.hosts) || projection.hosts.length > 100)) || (projection.navigation !== undefined && !isObject(projection.navigation))) throw new Error('The topology response has an unsupported or unbounded shape.'); }
  function isObject(value) { return Boolean(value && typeof value === 'object' && !Array.isArray(value)); }
  function clearProjectionUI(message) { if (!state.stage) return; if (state.stage.contains(document.activeElement)) { state.restoreGraphFocus = true; state.graph.focus({ preventScroll: true }); } state.focusCameraKey = ''; state.focusToggle.hidden = true; state.stage.replaceChildren(); state.drawing.setAttribute('viewBox', '0 0 1100 600'); state.caption.classList.remove('is-error'); state.caption.textContent = message; state.graphTitle.textContent = ({ l3: 'Layer 3 overview', zone: 'Zone overview', l2: 'Host & Layer 2 detail' })[state.view]; state.graph.setAttribute('aria-labelledby', `tl-tab-${state.view}`); state.pager.hidden = true; state.hostResultsList.replaceChildren(); state.hostResultsCount.textContent = state.busy ? 'Updating matching hosts…' : 'No host projection available.'; state.summary.replaceChildren(); for (const [label, value] of [['Hosts', '—'], ['Networks', '—'], ['Connections', '—'], ['On map', '—']]) { const cell = el('div', 'tl-stat'); cell.append(el('span', '', label), el('strong', '', value)); state.summary.append(cell); } state.traceStatus.replaceChildren(); state.sourceBody.replaceChildren(); document.getElementById('tl-sources').open = false; if (!state.dataset) state.options.replaceChildren(); state.exportButton.disabled = !state.dataset; if (!state.selected) renderInspector(null); syncInspectorView(); renderBreadcrumbs(); }
  function showError(error) { state.projection = null; state.focusHostId = ''; state.focusNotice = ''; state.selected = null; state.selectedEdge = null; renderInspector(null); syncInspectorView(); clearProjectionUI(`Could not load the network map: ${str(error && error.message, 'Request failed')}`); state.caption.classList.add('is-error'); }
  function drawTabs() { if (!state.tabs) return; state.tabs.querySelectorAll('[data-view]').forEach((b) => { const active = b.dataset.view === state.view; b.setAttribute('aria-selected', String(active)); b.tabIndex = active ? 0 : -1; }); if (state.graph) state.graph.setAttribute('aria-labelledby', `tl-tab-${state.view}`); }
  function chooseView(view) { if (!['l2', 'l3', 'zone'].includes(view) || view === state.view) return; state.view = view; state.offset = 0; drawTabs(); requestProjection(); }
  function clearPathDisplay() { state.traceRequested = false; state.focusPath = false; state.focusToggle.hidden = true; state.caption.textContent = 'Host selection changed · trace to update the L2 path.'; state.traceStatus.replaceChildren(); state.stage.querySelectorAll('.tl-node.tl-on-path').forEach((node) => node.classList.remove('tl-on-path')); state.stage.querySelectorAll('.tl-edge.tl-path-edge').forEach((edge) => edge.classList.remove('tl-path-edge')); }
  function projectionNodes() { const p = state.projection || {}; return arr(first(p.nodes, p.items, p.vertices, p.hosts)); }
  function projectionEdges() { const p = state.projection || {}; return arr(first(p.edges, p.links, p.connections)); }
  function layoutNodes(nodes, width, view) {
    const positions = new Map();
    if (view === 'l2' && nodes.length && nodes.every((item) => item.layout && Number.isFinite(Number(item.layout.x)) && Number.isFinite(Number(item.layout.y)))) {
      nodes.forEach((item) => { const layout = item.layout; positions.set(idOf(item), { x: Number(layout.x) + Number(layout.width || 168) / 2, y: Number(layout.y) + Number(layout.height || 84) / 2 }); });
      return positions;
    }
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
    const restoreGraphFocus = state.stage.contains(document.activeElement) || (state.restoreGraphFocus && document.activeElement === state.graph); state.restoreGraphFocus = false;
    const p = state.projection || {}; const allNodes = projectionNodes(); const allEdges = projectionEdges();
    reconcileSelection(allNodes);
    const hasPath = state.view === 'l2' && p.path && p.path.status === 'modeled_l2_path';
    const focused = hasPath && state.focusPath;
    const pathOrientation = focused && p.path_layout ? (state.graph.clientWidth < 850 ? 'vertical' : 'horizontal') : '';
    state.renderedPathOrientation = pathOrientation;
    state.graph.classList.toggle('is-focused-path', Boolean(pathOrientation));
    const pathNodes = new Set(arr(p.path && p.path.hops).map((hop) => hop.node_id));
    const pathLinks = new Set(arr(p.path && p.path.links));
    const nodes = focused ? allNodes.filter((item) => pathNodes.has(idOf(item))) : allNodes;
    const edges = focused ? allEdges.filter((edge) => pathLinks.has(edge.id)) : allEdges;
    state.focusToggle.hidden = !hasPath; state.focusToggle.textContent = focused ? 'Show all attachments' : 'Focus path'; state.focusToggle.setAttribute('aria-pressed', String(Boolean(focused)));
    const counts = p.counts || {}; const totalHosts = first(counts.hosts, p.total_hosts, p.host_count, '—'); const networks = first(counts.networks, p.network_count, '—'); const shown = nodes.length;
    state.summary.replaceChildren(); for (const [label, value] of [['Hosts', totalHosts], ['Networks', networks], ['Connections', allEdges.length], ['On map', shown]]) { const cell = el('div', 'tl-stat'); cell.append(el('span', '', label), el('strong', '', value)); state.summary.append(cell); }
    const viewLabels = { l3: 'Layer 3 overview', zone: 'Zone overview', l2: 'Host & Layer 2 detail' }; state.graphTitle.textContent = viewLabels[state.view];
    state.stage.replaceChildren(); state.focusCameraKey = '';
    if (!nodes.length) { state.drawing.setAttribute('viewBox', '0 0 1100 600'); state.caption.textContent = state.dataset ? (p.message || 'No elements match these filters. Remove a filter or reset filters to see results.') : 'Load the synthetic lab to explore this map.'; renderInspector(state.selected); renderPager(p); renderHostResults(p); renderBreadcrumbs(); state.exportButton.disabled = !state.dataset; state.preservePan = false; renderPath(p.path); syncGraphRover(); updateViewportControls(); if (restoreGraphFocus) state.graph.focus({ preventScroll: true }); return; }
    const rustLayout = state.view === 'l2' && p.layout && Number.isFinite(Number(p.layout.width)) && Number.isFinite(Number(p.layout.height)) ? p.layout : null;
    const activeLayout = pathOrientation && p.path_layout[pathOrientation] ? p.path_layout[pathOrientation] : null;
    const width = focused && activeLayout ? Number(activeLayout.width) : rustLayout ? Math.max(1100, Number(rustLayout.width)) : 1100; const hostRows = Math.ceil(nodes.filter((item) => typeOf(item) === 'host').length / 5); const height = focused && activeLayout ? Number(activeLayout.height) : rustLayout ? Math.max(600, Number(rustLayout.height)) : state.view === 'l2' ? Math.max(690, 280 + Math.max(0, hostRows - 1) * 82 + 68 + 32) : 600; state.drawing.setAttribute('viewBox', `0 0 ${width} ${height}`);
    if (focused) { const bounds = activeLayout ? focusedLayoutBounds(activeLayout) : layoutBounds(nodes, rustLayout); if (bounds) state.drawing.setAttribute('viewBox', `${bounds.x} ${bounds.y} ${bounds.width} ${bounds.height}`); }
    const positions = focused && activeLayout ? layoutPathNodes(nodes, activeLayout) : layoutNodes(nodes, width, state.view);
    edges.forEach((edge) => { const a = positions.get(str(first(edge.source_id, edge.source, edge.from, edge.source_host_id))); const b = positions.get(str(first(edge.destination_id, edge.target, edge.to, edge.destination_host_id))); if (!a || !b) return; const edgeId = edgeKey(edge); const selected = state.selected && state.selected.__edge && idOf(state.selected) === edgeId; const line = svg('path', { d: `M${a.x},${a.y} L${b.x},${b.y}`, class: `tl-edge ${edge.path ? 'tl-path-edge' : ''}${selected ? ' tl-selected-edge' : ''}`, 'data-link-id': edgeId, tabindex: '-1', role: 'button', 'aria-pressed': String(Boolean(selected)), 'aria-label': `Inspect connection ${edgeId || 'between endpoints'}` }); line.__edge = edge; line.addEventListener('click', () => selectEdge(edge)); line.addEventListener('keydown', (event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectEdge(edge); } }); state.stage.append(line); });
    nodes.forEach((item) => { const point = positions.get(idOf(item)); const kind = typeOf(item); const selected = state.selected && idOf(state.selected) === idOf(item) && typeOf(state.selected) === kind; const group = svg('g', { class: `tl-node tl-kind-${kind}${selected ? ' tl-selected' : ''}`, tabindex: '-1', role: 'button', 'aria-pressed': String(Boolean(selected)), 'aria-label': `${kind}: ${labelOf(item)}${selected ? ', selected' : ''}`, transform: `translate(${point.x} ${point.y})` }); group.__item = item;
      const box = svg('rect', { x: '-80', y: '-34', width: '160', height: '68', rx: '12', class: 'tl-node-box' }); const title = svg('text', { x: '-42', y: '-2', class: 'tl-node-title' }); title.textContent = clipped(labelOf(item), 19); const sub = svg('text', { x: '-42', y: '17', class: 'tl-node-subtitle' }); sub.textContent = clipped(str(first(item.subtitle, item.cidr, item.ip, item.host_count && `${item.host_count} hosts`), kind), 23); group.append(box, makeIcon(kind, item), title, sub); state.stage.append(group);
    });
    const total = Number(first(p.pagination && p.pagination.total, p.total, p.total_count, p.filtered_count, allNodes.length));
    state.caption.textContent = str(first(p.caption, p.message), `${shown} elements in this projection · ${allEdges.length} projected connections · configuration-derived map · ${totalHosts} hosts in the synthetic dataset · live observations unavailable`);
    if (focused) state.caption.textContent = `${shown} of ${allNodes.length} elements · selected L2 path · live observations unavailable`;
    state.exportButton.disabled = !state.dataset;
    renderPager(p); if (focused) state.pager.hidden = true; renderHostOptions(nodes); renderHostResults(p); renderBreadcrumbs(); renderInspector(state.selected); if (!state.preservePan) { state.scale = 1; state.x = 0; state.y = 0; } state.preservePan = false;
    renderMinimap(nodes, edges, positions); syncGraphRover(null, restoreGraphFocus); applyTransform(); renderPath(p.path); updateViewportControls();
  }
  function renderPager(p) { const page = p.navigation || {}; const total = Number(first(p.pagination && p.pagination.total, p.total, p.total_count, p.filtered_count, 0)); const start = total ? state.offset + 1 : 0; const end = total ? Math.min(state.offset + arr(p.hosts).length, total) : 0; state.pageLabel.textContent = `Hosts ${start}–${end} of ${total}`; state.pagePosition.textContent = total ? `Page ${page.page_index || Math.floor(state.offset / state.limit) + 1} of ${page.page_count || Math.ceil(total / state.limit)}` : 'Page 0 of 0'; state.prev.disabled = state.busy || page.previous_offset == null; state.next.disabled = state.busy || page.next_offset == null; state.firstPage.disabled = state.busy || page.previous_offset == null; state.lastPage.disabled = state.busy || page.next_offset == null; state.pager.hidden = state.view === 'l2' && state.focusPath && p.path && p.path.status === 'modeled_l2_path'; }
  function renderHostResults(p) {
    const hosts = arr(p.hosts); const pagination = p.pagination || {}; const total = Number(first(p.counts && p.counts.matched_hosts, pagination.total, p.total, hosts.length)); const datasetHosts = Number(first(p.counts && p.counts.hosts, p.counts && p.counts.dataset_hosts, state.dataset && state.dataset.hosts && state.dataset.hosts.length, 0));
    state.hostResultsCount.textContent = `${state.focusNotice ? `${state.focusNotice} · ` : ''}${total ? `Showing ${state.offset + 1}–${state.offset + hosts.length} of ${total} matching hosts` : 'No hosts match these filters'} · ${datasetHosts} in dataset`;
    state.hostResultsList.replaceChildren();
    hosts.forEach((host) => {
      const row = el('article', 'tl-host-result'); row.dataset.hostId = idOf(host); row.setAttribute('role', 'listitem');
      const details = el('div', 'tl-host-result-details'); details.append(el('strong', '', labelOf(host)), el('span', '', [first(host.ip, host.ip_address), first(host.network_id, host.network), first(host.zone_id, host.zone), host.vrf && `VRF ${host.vrf}`, host.vdom && `VDOM ${host.vdom}`].filter(Boolean).join(' · ')), el('small', '', `ID ${idOf(host)}${first(host.mac, host.mac_address) ? ` · MAC ${first(host.mac, host.mac_address)}` : ''}`));
      const actions = el('div', 'tl-host-result-actions');
      for (const [action, label] of [['inspect', 'Inspect'], ['map', 'Show on map'], ['source', 'Use as source'], ['destination', 'Use as destination']]) { const b = button('', label, 'tl-button'); b.dataset.action = action; actions.append(b); }
      row.append(details, actions); state.hostResultsList.append(row);
    });
    if (!hosts.length) { const empty = el('p', 'tl-host-results-empty', 'Try removing a filter or broadening the search.'); empty.setAttribute('role', 'status'); state.hostResultsList.append(empty); }
  }
  function renderBreadcrumbs() { state.breadcrumbs.querySelectorAll('.tl-breadcrumb-current').forEach((node) => node.remove()); const bits = [({ l3: 'Layer 3', zone: 'Zones', l2: 'Host & Layer 2' })[state.view], state.network && `Network ${state.network}`, state.zone && `Zone ${state.zone}`, state.vrf && `VRF ${state.vrf}`, state.vdom && `VDOM ${state.vdom}`, state.search && `Search “${state.search}”`, state.selected && typeOf(state.selected) === 'host' && labelOf(state.selected)].filter(Boolean); state.breadcrumbs.append(el('span', 'tl-breadcrumb-current', bits.join(' › '))); state.back.disabled = state.backStack.length === 0 || state.busy; }
  function saveNavigation() { state.backStack.push({ view: state.view, network: state.network, zone: state.zone, vrf: state.vrf, vdom: state.vdom, search: state.search, offset: state.offset, limit: state.limit, selected: state.selected, selectedEdge: state.selectedEdge, inspectorCollapsed: state.inspectorCollapsed, scale: state.scale, x: state.x, y: state.y }); if (state.backStack.length > 20) state.backStack.shift(); }
  function navigateBack() { const prior = state.backStack.pop(); if (!prior) return; Object.assign(state, prior, { preservePan: true }); state.net.value = state.network; state.zoneInput.value = state.zone; state.vrfInput.value = state.vrf; state.vdomInput.value = state.vdom; state.searchInput.value = state.search; state.pageSize.value = String(state.limit); drawTabs(); renderFilters(); requestProjection(); }
  function navigatePage(offset) { if (!Number.isFinite(offset) || offset === state.offset || state.busy) return; state.offset = Math.max(0, offset); state.preservePan = true; requestProjection(); }
  function inspectHost(host) { state.selected = host; state.selectedEdge = null; state.inspectorCollapsed = false; renderInspector(host); syncInspectorView(); renderBreadcrumbs(); }
  function focusHostOnMap(host) { saveNavigation(); state.view = 'l2'; state.network = str(first(host.network_id, host.network), state.network); state.net.value = state.network; state.offset = 0; state.focusHostId = idOf(host); state.focusNotice = ''; state.selected = null; state.preservePan = false; drawTabs(); renderFilters(); requestProjection(); }
  function layoutBounds(nodes, fallback) {
    const cards = nodes.map((item) => item.layout).filter((layout) => layout && Number.isFinite(Number(layout.x)) && Number.isFinite(Number(layout.y)));
    if (!cards.length) return fallback ? { x: 0, y: 0, width: Number(fallback.width), height: Number(fallback.height) } : null;
    const left = Math.min(...cards.map((layout) => Number(layout.x))); const top = Math.min(...cards.map((layout) => Number(layout.y)));
    const right = Math.max(...cards.map((layout) => Number(layout.x) + Number(layout.width || 168))); const bottom = Math.max(...cards.map((layout) => Number(layout.y) + Number(layout.height || 84)));
    const padding = 56; return { x: left - padding, y: top - padding, width: Math.max(360, right - left + padding * 2), height: Math.max(260, bottom - top + padding * 2) };
  }
  function focusedLayoutBounds(layout) { if (!layout || !Number.isFinite(Number(layout.width)) || !Number.isFinite(Number(layout.height))) return null; const padding = 28; return { x: -padding, y: -padding, width: Number(layout.width) + padding * 2, height: Number(layout.height) + padding * 2 }; }
  function layoutPathNodes(nodes, layout) { const positions = new Map(); const layouts = new Map(arr(layout.nodes).map((item) => [str(item.id), item.layout])); nodes.forEach((item) => { const card = layouts.get(idOf(item)); if (card && Number.isFinite(Number(card.x)) && Number.isFinite(Number(card.y))) positions.set(idOf(item), { x: Number(card.x) + Number(card.width || 168) / 2, y: Number(card.y) + Number(card.height || 84) / 2 }); }); return positions; }
  function renderHostOptions(nodes) { state.options.replaceChildren(); const candidates = [...arr(state.dataset && state.dataset.hosts), ...nodes, ...arr(state.projection && state.projection.hosts)]; const seen = new Set(); candidates.filter((item) => typeOf(item) === 'host' && !seen.has(idOf(item)) && seen.add(idOf(item))).forEach((item) => { const option = el('option'); option.value = `${labelOf(item)} (${idOf(item)})`; option.dataset.id = idOf(item); option.dataset.host = labelOf(item); state.options.append(option); }); }
  function findHostId(value) { const match = [...state.options.options].find((o) => o.value === value || o.dataset.id === value || o.dataset.host === value); if (match) return match.dataset.id; const host = arr(state.dataset && state.dataset.hosts).find((item) => item.id === value || item.ip === value || item.mac === value); return host ? host.id : ''; }
  function selectItem(item, options = {}) { state.selected = item; state.selectedEdge = null; state.inspectorCollapsed = false; syncInspectorView(); refreshNodeSelection(); refreshEdgeSelection(); renderInspector(item); const kind = typeOf(item); if (kind === 'network' || kind === 'subnet') { state.network = idOf(item); state.net.value = state.network; state.zone = ''; state.zoneInput.value = ''; state.view = 'l2'; state.offset = 0; state.preservePan = true; drawTabs(); renderFilters(); requestProjection(); } else if (kind === 'zone') { state.zone = str(item.zone, idOf(item).replace(/^zone-/, '')); state.zoneInput.value = state.zone; state.network = ''; state.net.value = ''; state.view = 'l3'; state.offset = 0; state.preservePan = true; drawTabs(); renderFilters(); requestProjection(); } else if (kind === 'host' && !options.preserveTrace) { if (!state.sourcePinned) { state.source = idOf(item); state.sourceField.input.value = `${labelOf(item)} (${idOf(item)})`; } clearPathDisplay(); } }
  function reconcileSelection(nodes) { if (!state.selected) { syncInspectorView(); return; } if (state.selected.__edge) { const id = idOf(state.selected); const edge = projectionEdges().find((candidate) => edgeKey(candidate) === id); if (edge) { state.selected.__edgeData = edge; renderInspector(state.selected); } else { state.selected = null; state.selectedEdge = null; renderInspector(null); refreshEdgeSelection(); } syncInspectorView(); return; } const id = idOf(state.selected); const kind = typeOf(state.selected); const candidates = kind === 'host' ? [...nodes, ...arr(state.projection && state.projection.hosts)] : nodes; const match = candidates.find((item) => idOf(item) === id && typeOf(item) === kind); state.selected = match || null; if (match) renderInspector(match); else renderInspector(null); refreshEdgeSelection(); syncInspectorView(); }
  function syncInspectorView() { if (!state.content || !state.inspector) return; const hasSelection = Boolean(state.selected); const collapsed = !hasSelection || state.inspectorCollapsed; state.content.classList.toggle('has-selection', hasSelection); state.content.classList.toggle('is-inspector-collapsed', collapsed); state.inspector.setAttribute('aria-hidden', String(collapsed)); state.inspectorToggle.disabled = !hasSelection; state.inspectorToggle.textContent = collapsed ? 'Show details' : 'Hide details'; state.inspectorToggle.setAttribute('aria-expanded', String(!collapsed)); scheduleResponsivePathLayout(); }
  function refreshNodeSelection() { if (!state.stage) return; state.stage.querySelectorAll('.tl-node').forEach((node) => { const selected = state.selected && !state.selected.__edge && idOf(state.selected) === idOf(node.__item) && typeOf(state.selected) === typeOf(node.__item); node.classList.toggle('tl-selected', Boolean(selected)); node.setAttribute('aria-pressed', String(Boolean(selected))); node.setAttribute('aria-label', `${typeOf(node.__item)}: ${labelOf(node.__item)}${selected ? ', selected' : ''}`); }); syncGraphRover(); updateViewportControls(); }
  function refreshEdgeSelection() { if (!state.stage) return; state.stage.querySelectorAll('.tl-edge').forEach((edge) => { const selected = Boolean(state.selected && state.selected.__edge && state.selected.__edgeData && edgeKey(state.selected.__edgeData) === edge.dataset.linkId); edge.classList.toggle('tl-selected-edge', selected); edge.setAttribute('aria-pressed', String(selected)); }); syncGraphRover(); updateViewportControls(); }
  function renderInspector(item) {
    state.inspector.replaceChildren();
    if (!item) { state.inspector.append(el('div', 'tl-empty', 'Select a network, zone, or host to inspect its details.')); return; }
    if (item.__edge) { renderEdgeInspector(item.__edgeData || item.edge); return; }
    const kind = typeOf(item); state.inspector.append(el('span', 'tl-eyebrow', `${kind} detail`), el('h3', '', labelOf(item)));
    const selectionActions = el('div', 'tl-selection-actions');
    const clearSelection = button('tl-clear-selection', 'Clear selection'); clearSelection.addEventListener('click', () => { state.selected = null; state.selectedEdge = null; syncInspectorView(); refreshNodeSelection(); refreshEdgeSelection(); renderInspector(null); }); selectionActions.append(clearSelection);
    if (kind === 'host') {
      const sourceButton = button('', 'Use as source'); sourceButton.addEventListener('click', () => setTraceEndpoint('source', item));
      const destinationButton = button('', 'Use as destination'); destinationButton.addEventListener('click', () => setTraceEndpoint('destination', item));
      selectionActions.append(sourceButton, destinationButton);
    }
    state.inspector.append(selectionActions);
    const badge = el('span', 'tl-source-badge', str(first(item.source_badge, item.source, 'Synthetic lab dataset'))); state.inspector.append(badge);
    const list = el('dl', 'tl-details'); const data = kind === 'host' ? [['Host ID', idOf(item)], ['IP address', first(item.ip, item.ip_address, item.address)], ['MAC address', first(item.mac, item.mac_address)], ['Network', first(item.network_id, item.network)], ['Zone', first(item.zone_id, item.zone)], ['VLAN', first(item.vlan, item.vlan_id)], ['VRF', item.vrf], ['VDOM', item.vdom], ['Port', first(item.port, item.ports && portsText(item.ports))], ['Attachment', item.attachment]] : [['ID', idOf(item)], ['Type', first(item.role, item.type, item.kind)], ['CIDR', first(item.cidr, item.subnet, item.prefix)], ['Network ID', item.network_id], ['IP address', first(item.ip, item.ip_address)], ['Zone', first(item.zone_id, item.zone)], ['VLAN', first(item.vlan, item.vlan_id)], ['VRF', item.vrf], ['VDOM', item.vdom], ['Ports', item.ports && portsText(item.ports)], ['Hosts', first(item.host_count, item.count)]];
    data.filter(([, value]) => value !== undefined && value !== null && value !== '').forEach(([k, v]) => { list.append(el('dt', '', k), el('dd', '', Array.isArray(v) ? v.join(', ') : String(v))); }); state.inspector.append(list);
    const itemId = idOf(item); const connections = projectionEdges().filter((edge) => str(first(edge.source_id, edge.source, edge.from, edge.source_host_id)) === itemId || str(first(edge.destination_id, edge.target, edge.to, edge.destination_host_id)) === itemId);
    const connectionSection = el('section', 'tl-connections'); connectionSection.append(el('h4', '', `Connections · ${connections.length}`));
    if (connections.length) {
      const rows = el('ul', 'tl-connection-list');
      connections.slice(0, 12).forEach((edge) => { const from = str(first(edge.source_id, edge.source, edge.from, edge.source_host_id)); const to = str(first(edge.destination_id, edge.target, edge.to, edge.destination_host_id)); const peerId = from === itemId ? to : from; const peer = projectionNodes().find((node) => idOf(node) === peerId); const label = peer ? labelOf(peer) : peerId; const localPort = str(first(from === itemId ? edge.from_port : edge.to_port, from === itemId ? edge.source_port : edge.destination_port)); const remotePort = str(first(from === itemId ? edge.to_port : edge.from_port, from === itemId ? edge.destination_port : edge.source_port)); const detail = first(edge.label, edge.kind, edge.type, edge.protocol); const row = el('li'); const navigate = button('', `${from === itemId ? '→' : '←'} ${label}`, 'tl-link-nav'); navigate.setAttribute('aria-label', `Inspect neighbor ${label} on ${localPort || 'connected port'}`); navigate.addEventListener('click', () => { if (peer) selectItem(peer); }); navigate.disabled = !peer; row.append(navigate); const inspect = button('', 'Ports & link', 'tl-link-nav tl-link-detail'); inspect.addEventListener('click', () => selectEdge(edge)); row.append(inspect); if (detail) row.append(el('small', '', String(detail))); rows.append(row); });
      if (connections.length > 12) rows.append(el('li', 'tl-muted', `${connections.length - 12} more connections in the projection`)); connectionSection.append(rows);
    } else connectionSection.append(el('p', 'tl-muted', 'No connections are present in this projection.'));
    state.inspector.append(connectionSection);
    if (connections.length) {
      const connectedPorts = el('section', 'tl-port-section'); connectedPorts.append(el('h4', '', `Connected ports · ${connections.length}`));
      const connectedList = el('ul', 'tl-port-list');
      connections.slice(0, 24).forEach((edge) => {
        const from = str(first(edge.source_id, edge.source, edge.from, edge.source_host_id)); const to = str(first(edge.destination_id, edge.target, edge.to, edge.destination_host_id));
        const peerId = from === itemId ? to : from; const peer = projectionNodes().find((node) => idOf(node) === peerId); const localPort = str(first(from === itemId ? edge.from_port : edge.to_port, from === itemId ? edge.source_port : edge.destination_port), 'Port unavailable');
        const row = el('li'); const inspectNeighbor = button('', `${localPort} → ${peer ? labelOf(peer) : peerId}`, peer ? 'tl-link-nav' : 'tl-port-label'); inspectNeighbor.setAttribute('aria-label', `Inspect neighbor ${peer ? labelOf(peer) : peerId} connected on ${localPort}`); if (peer) inspectNeighbor.addEventListener('click', () => selectItem(peer)); else inspectNeighbor.disabled = true;
        row.append(inspectNeighbor); const inspectLink = button('', 'Inspect link ports', 'tl-link-nav tl-link-detail'); inspectLink.addEventListener('click', () => selectEdge(edge)); row.append(inspectLink); connectedList.append(row);
      });
      if (connections.length > 24) connectedList.append(el('li', 'tl-muted', `${connections.length - 24} more connected ports omitted`));
      connectedPorts.append(connectedList); state.inspector.append(connectedPorts);
    }
    const ports = arr(item.ports);
    if (ports.length) {
      const portSection = el('section', 'tl-port-section'); portSection.append(el('h4', '', `Ports · ${ports.length}`));
      const portList = el('ul', 'tl-port-list'); ports.slice(0, 24).forEach((port) => { const row = el('li'); const label = typeof port === 'object' ? portsText([port]) : String(port); const peerId = typeof port === 'object' ? str(first(port.peer_id, port.neighbor_id, port.remote_node_id, port.connected_to)) : ''; const peer = peerId && projectionNodes().find((node) => idOf(node) === peerId); const detail = button('', peer ? `${label} → ${labelOf(peer)}` : label, peer ? 'tl-link-nav' : 'tl-port-label'); if (peer) detail.addEventListener('click', () => selectItem(peer)); row.append(detail); if (typeof port === 'object' && first(port.vrf, port.vdom, port.status)) row.append(el('small', '', [port.vrf && `VRF ${port.vrf}`, port.vdom && `VDOM ${port.vdom}`, port.status].filter(Boolean).join(' · '))); portList.append(row); });
      if (ports.length > 24) portList.append(el('li', 'tl-muted', `${ports.length - 24} more ports omitted`)); portSection.append(portList); state.inspector.append(portSection);
    }
    const evidence = arr(first(item.evidence, item.sources, item.source_lines)); if (evidence.length) { const btn = button('tl-evidence-button', `View ${evidence.length} source references`); btn.addEventListener('click', () => showSources(resolveSources(evidence))); state.inspector.append(btn); }
  }
  function selectEdge(edge) {
    const edgeId = edgeKey(edge);
    state.selected = { id: edgeId, label: `Connection ${edgeId}`, role: 'connection', __edge: true, __edgeData: edge };
    state.selectedEdge = edge; state.inspectorCollapsed = false; syncInspectorView(); refreshNodeSelection(); refreshEdgeSelection(); renderInspector(state.selected);
  }
  function edgeKey(edge) { return str(first(edge.id, edge.link_id, `${first(edge.source_id, edge.from, edge.source, edge.source_host_id)}-${first(edge.destination_id, edge.to, edge.target, edge.destination_host_id)}`)); }
  function renderEdgeInspector(edge) {
    if (!edge) return;
    const fromId = str(first(edge.source_id, edge.source, edge.from, edge.source_host_id)); const toId = str(first(edge.destination_id, edge.target, edge.to, edge.destination_host_id));
    const from = projectionNodes().find((node) => idOf(node) === fromId); const to = projectionNodes().find((node) => idOf(node) === toId);
    state.inspector.append(el('span', 'tl-eyebrow', 'connection detail'), el('h3', '', `${labelOf(from || { id: fromId })} → ${labelOf(to || { id: toId })}`));
    const clear = button('tl-clear-selection', 'Clear selection'); clear.addEventListener('click', () => { state.selected = null; state.selectedEdge = null; syncInspectorView(); refreshNodeSelection(); refreshEdgeSelection(); renderInspector(null); }); state.inspector.append(clear);
    const localPort = first(edge.from_port, edge.source_port, edge.local_port);
    const remotePort = first(edge.to_port, edge.destination_port, edge.remote_port);
    const list = el('dl', 'tl-details'); const data = [['Connection ID', first(edge.id, edge.link_id)], ['Type', first(edge.label, edge.kind, edge.type, edge.protocol)], ['Local port', localPort], ['Remote port', remotePort], ['VLAN', first(edge.vlan, edge.vlan_id)], ['VRF', edge.vrf], ['VDOM', edge.vdom]];
    data.filter(([, value]) => value !== undefined && value !== null && value !== '').forEach(([key, value]) => list.append(el('dt', '', key), el('dd', '', String(value)))); state.inspector.append(list);
    const evidence = arr(first(edge.evidence, edge.sources, edge.source_lines)); if (evidence.length) { const btn = button('tl-evidence-button', `View ${evidence.length} source references`); btn.addEventListener('click', () => showSources(resolveSources(evidence))); state.inspector.append(btn); }
  }
  function setTraceEndpoint(which, item) {
    const value = `${labelOf(item)} (${idOf(item)})`;
    if (which === 'source') { state.source = idOf(item); state.sourcePinned = true; state.sourceField.input.value = value; }
    else { state.destination = idOf(item); state.destinationField.input.value = value; }
    clearPathDisplay();
    state.traceStatus.textContent = `${which === 'source' ? 'Source' : 'Destination'} host selected. Choose the other endpoint, then trace.`;
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
      hops.slice(0, 100).forEach((hop, index) => {
        const entry = el('li');
        const hopId = str(first(hop.host_id, hop.node_id, hop.id)); const hopNode = projectionNodes().find((node) => idOf(node) === hopId);
        const hopSelect = button('', str(first(hop.label, hop.name, hop.node_id, hop.id)), hopNode ? 'tl-link-nav tl-hop-select' : 'tl-hop-select');
        if (hopNode) hopSelect.addEventListener('click', () => selectItem(hopNode, { preserveTrace: true })); else hopSelect.disabled = true;
        entry.append(hopSelect);
        const portText = [`${str(hop.ingress, 'Origin')} → ${str(hop.egress, 'Destination')}`, hop.vlan !== undefined && `VLAN ${hop.vlan}`, hop.vrf && `VRF ${hop.vrf}`, hop.vdom && `VDOM ${hop.vdom}`].filter(Boolean).join(' · ');
        entry.append(el('span', 'tl-hop-ports', portText));
        const links = arr(path.links); const adjacentIds = [links[index - 1], links[index]].filter(Boolean).map((link) => typeof link === 'object' ? str(first(link.id, link.link_id)) : String(link));
        const adjacent = projectionEdges().filter((edge) => adjacentIds.includes(str(first(edge.id, edge.link_id))));
        const actions = el('div', 'tl-hop-actions');
        adjacent.forEach((edge) => { const linkId = str(first(edge.id, edge.link_id)); const direction = index > 0 && adjacentIds.indexOf(linkId) === 0 ? 'Ingress' : 'Egress'; const from = str(first(edge.from, edge.source_id, edge.source)); const to = str(first(edge.to, edge.destination_id, edge.target)); const localPort = str(first(from === hopId ? edge.from_port : edge.to_port, from === hopId ? edge.source_port : edge.destination_port), 'local port'); const remotePort = str(first(from === hopId ? edge.to_port : edge.from_port, from === hopId ? edge.destination_port : edge.source_port), 'peer port'); const inspect = button('', `Inspect ${direction} · ${localPort} ↔ ${remotePort}`, 'tl-button tl-hop-link'); inspect.addEventListener('click', () => selectEdge(edge)); actions.append(inspect); });
        if (arr(hop.evidence).length) {
          const source = button('', 'View hop sources', 'tl-button tl-hop-source');
          source.setAttribute('aria-label', `Source evidence for ${str(hop.node_id)}`);
          source.addEventListener('click', () => showSources(resolveSources(hop.evidence)));
          actions.append(source);
        }
        if (actions.childElementCount) entry.append(actions);
        list.append(entry);
      });
      if (hops.length > 100) list.append(el('li', 'tl-muted', `${hops.length - 100} additional hops omitted`));
      state.traceStatus.append(list);
    }
    if (path.status === 'requires_l3') state.traceStatus.append(el('span', 'tl-callout', 'These hosts are on different networks or VLANs; an L2 path cannot be established.'));
    else if (path.status === 'indeterminate' && path.reason === 'outside_selected_context') state.traceStatus.append(el('span', 'tl-callout', 'One or both selected endpoints are outside the current VRF/VDOM and filter context. Clear or change the scope to trace them.'));
    else if (path.status === 'indeterminate') state.traceStatus.append(el('span', 'tl-callout', 'The available link evidence does not establish a path.'));
    state.traceStatus.append(el('span', 'tl-callout', 'Runtime observations unavailable; this result uses the synthetic lab configuration.'));
    const pathIds = new Set(hops.map((hop) => str(first(hop.host_id, hop.id, hop.node_id, hop)))); state.stage.querySelectorAll('.tl-node').forEach((node) => { if (pathIds.has(idOf(node.__item))) node.classList.add('tl-on-path'); });
    const linkIds = new Set(arr(path.links).map(String)); state.stage.querySelectorAll('.tl-edge').forEach((edge) => { if (linkIds.has(edge.dataset.linkId)) edge.classList.add('tl-path-edge'); });
  }
  async function requestPath() {
    const s = findHostId(state.sourceField.input.value); const d = findHostId(state.destinationField.input.value);
    if (!s || !d || s === d) { clearPathDisplay(); state.traceStatus.textContent = 'Choose two distinct lab hosts by name or ID.'; return; }
    state.source = s; state.destination = d;
    state.focusPath = true; state.traceRequested = true;
    const sourceHost = arr(state.dataset && state.dataset.hosts).find((host) => host.id === s);
    if (sourceHost) { state.view = 'l2'; state.network = sourceHost.network_id; state.net.value = state.network; state.zone = ''; state.zoneInput.value = ''; state.offset = 0; drawTabs(); }
    const projection = await requestProjection(false);
    if (!projection) return;
    if (!projection.path) state.traceStatus.textContent = 'The current projection did not include a path result.';
  }
  // These controls navigate the rendered projection only. They never infer
  // neighbors, change scope, or submit a new analysis request.
  function graphItems() { return state.stage ? [...state.stage.querySelectorAll('.tl-node,.tl-edge')] : []; }
  function graphItemKey(item) { return item && (item.__item ? `node:${idOf(item.__item)}` : item.__edge ? `edge:${edgeKey(item.__edge)}` : ''); }
  function selectedGraphItem() {
    if (!state.selected) return null;
    const key = state.selected.__edge ? `edge:${idOf(state.selected)}` : `node:${idOf(state.selected)}`;
    return graphItems().find((item) => graphItemKey(item) === key) || null;
  }
  function syncGraphRover(preferred, focus = false) {
    const items = graphItems();
    const current = preferred || items.find((item) => graphItemKey(item) === state.rovingKey) || selectedGraphItem() || items[0];
    state.rovingKey = graphItemKey(current) || '';
    items.forEach((item) => item.setAttribute('tabindex', item === current ? '0' : '-1'));
    if (focus && current) current.focus({ preventScroll: true });
  }
  function navigateGraphItems(event) {
    const current = event.target.closest('.tl-node,.tl-edge');
    if (!current || !state.stage.contains(current)) return;
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); state.graph.focus({ preventScroll: true }); return; }
    const items = graphItems(); const index = items.indexOf(current); let next;
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = items[(index + 1) % items.length];
    else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = items[(index + items.length - 1) % items.length];
    else if (event.key === 'Home') next = items[0];
    else if (event.key === 'End') next = items.at(-1);
    else return;
    event.preventDefault(); event.stopPropagation(); syncGraphRover(next, true); focusGraphItem(next);
  }
  function updateViewportControls() {
    if (!state.focusSelection) return;
    state.focusSelection.disabled = state.busy || !selectedGraphItem();
    state.minimap.toggleAttribute('hidden', state.busy || !state.projection || !state.stage.querySelector('.tl-node'));
    const selectedKey = graphItemKey(selectedGraphItem());
    state.miniContent.querySelectorAll('[data-mini-key]').forEach((item) => item.classList.toggle('is-selected', item.dataset.miniKey === selectedKey));
    updateMinimapViewport();
  }
  function visibleDrawingBounds() {
    const matrix = state.drawing.getScreenCTM(); if (!matrix) return null;
    const rect = state.drawing.getBoundingClientRect(); const inverse = matrix.inverse();
    const point = state.drawing.createSVGPoint(); point.x = rect.left; point.y = rect.top; const top = point.matrixTransform(inverse);
    point.x = rect.right; point.y = rect.bottom; const bottom = point.matrixTransform(inverse);
    return { x: top.x, y: top.y, width: bottom.x - top.x, height: bottom.y - top.y };
  }
  function renderedItemBounds(item) {
    const stageMatrix = state.stage.getScreenCTM(); const itemMatrix = item && item.getScreenCTM();
    if (!stageMatrix || !itemMatrix) return null;
    const transform = stageMatrix.inverse().multiply(itemMatrix); const box = item.getBBox();
    const point = state.drawing.createSVGPoint(); point.x = box.x; point.y = box.y; const top = point.matrixTransform(transform);
    point.x = box.x + box.width; point.y = box.y + box.height; const bottom = point.matrixTransform(transform);
    return { x: top.x, y: top.y, width: bottom.x - top.x, height: bottom.y - top.y };
  }
  function focusGraphItem(item) {
    if (!item || state.busy) return;
    const bounds = renderedItemBounds(item); const visible = visibleDrawingBounds(); const matrix = state.drawing.getScreenCTM();
    if (!bounds || !visible || !matrix) return;
    const screenScale = Math.min(1.25, Math.max(1, state.graph.clientWidth - 48) / Math.max(160, bounds.width), Math.max(1, state.graph.clientHeight - 64) / Math.max(68, bounds.height));
    state.scale = Math.max(.35, Math.min(12, screenScale / Math.hypot(matrix.a, matrix.b)));
    state.x = visible.x + visible.width / 2 - (bounds.x + bounds.width / 2) * state.scale;
    state.y = visible.y + visible.height / 2 - (bounds.y + bounds.height / 2) * state.scale;
    state.focusCameraKey = graphItemKey(item); scheduleTransform();
  }
  function focusPendingHost() {
    if (!state.focusCameraTarget) return;
    const target = graphItems().find((item) => item.__item && idOf(item.__item) === state.focusCameraTarget);
    state.focusCameraTarget = '';
    if (target && state.projection && state.projection.navigation?.focus_status === 'visible') focusAndCenterGraphItem(target);
  }
  function focusAndCenterGraphItem(item) {
    if (!item || state.busy) return;
    syncGraphRover(item, true);
    focusGraphItem(item);
  }
  function renderMinimap(nodes, edges, positions) {
    state.miniContent.replaceChildren(); state.minimap.setAttribute('viewBox', state.drawing.getAttribute('viewBox'));
    edges.forEach((edge) => {
      const a = positions.get(str(first(edge.source_id, edge.source, edge.from, edge.source_host_id)));
      const b = positions.get(str(first(edge.destination_id, edge.target, edge.to, edge.destination_host_id)));
      if (a && b) state.miniContent.append(svg('path', { d: `M${a.x},${a.y} L${b.x},${b.y}`, class: 'tl-mini-edge' }));
    });
    nodes.forEach((item) => {
      const position = positions.get(idOf(item)); if (!position) return;
      const node = svg('rect', { x: position.x - 64, y: position.y - 28, width: 128, height: 56, rx: 8, class: `tl-mini-node tl-mini-${typeOf(item)}`, 'data-mini-key': `node:${idOf(item)}` });
      const title = svg('title'); title.textContent = `Focus ${typeOf(item)} ${labelOf(item)}`; node.append(title); state.miniContent.append(node);
    });
  }
  function updateMinimapViewport() {
    if (!state.minimap || state.minimap.hasAttribute('hidden')) return;
    const visible = visibleDrawingBounds(); if (!visible) return;
    state.miniViewport.setAttribute('x', (visible.x - state.x) / state.scale);
    state.miniViewport.setAttribute('y', (visible.y - state.y) / state.scale);
    state.miniViewport.setAttribute('width', visible.width / state.scale);
    state.miniViewport.setAttribute('height', visible.height / state.scale);
  }
  function recenterFromOverview(event) {
    event.preventDefault(); event.stopPropagation();
    const matrix = state.minimap.getScreenCTM(); const visible = visibleDrawingBounds(); if (!matrix || !visible || state.busy) return;
    const point = state.drawing.createSVGPoint(); point.x = event.clientX; point.y = event.clientY; const center = point.matrixTransform(matrix.inverse());
    state.focusCameraKey = ''; state.x = visible.x + visible.width / 2 - center.x * state.scale; state.y = visible.y + visible.height / 2 - center.y * state.scale; scheduleTransform();
  }
  function navigateOverview(event) {
    const visible = visibleDrawingBounds(); if (!visible || state.busy) return;
    if (event.key === 'Enter' || event.key === ' ' || event.key === 'Home') fitGraph();
    else if (event.key === 'ArrowLeft') state.x += visible.width / 10;
    else if (event.key === 'ArrowRight') state.x -= visible.width / 10;
    else if (event.key === 'ArrowUp') state.y += visible.height / 10;
    else if (event.key === 'ArrowDown') state.y -= visible.height / 10;
    else return;
    event.preventDefault(); event.stopPropagation(); state.focusCameraKey = ''; scheduleTransform();
  }
  function fitGraph() { state.focusCameraKey = ''; state.scale = 1; state.x = 0; state.y = 0; scheduleTransform(); }
  function setZoom(value) { const box = state.drawing && state.drawing.viewBox.baseVal; zoomAt(value, box ? box.x + box.width / 2 : 550, box ? box.y + box.height / 2 : 300); }
  function zoomAt(value, cx, cy) { state.focusCameraKey = ''; const next = Math.max(.35, Math.min(12, value)); const ratio = next / state.scale; state.x = cx - (cx - state.x) * ratio; state.y = cy - (cy - state.y) * ratio; state.scale = next; scheduleTransform(); }
  function screenPoint(event) { const matrix = state.drawing && state.drawing.getScreenCTM(); if (matrix) { const point = state.drawing.createSVGPoint(); point.x = event.clientX; point.y = event.clientY; const converted = point.matrixTransform(matrix.inverse()); return { x: converted.x, y: converted.y }; } const rect = state.graph.getBoundingClientRect(); const box = state.drawing.viewBox.baseVal; return { x: box.x + (event.clientX - rect.left) * box.width / rect.width, y: box.y + (event.clientY - rect.top) * box.height / rect.height }; }
  function scheduleTransform() { if (!state.stage || state.transformFrame) return; state.transformFrame = requestAnimationFrame(() => { state.transformFrame = 0; applyTransform(); }); }
  function scheduleResponsivePathLayout() { if (state.responsivePathFrame) return; state.responsivePathFrame = requestAnimationFrame(() => { state.responsivePathFrame = 0; const projection = state.projection || {}; const hasFocusedPath = state.focusPath && state.view === 'l2' && projection.path && projection.path.status === 'modeled_l2_path' && projection.path_layout; const desired = hasFocusedPath ? (state.graph.clientWidth < 850 ? 'vertical' : 'horizontal') : ''; if (desired && desired !== state.renderedPathOrientation) { state.preservePan = false; render(); return; } if (!desired) { state.renderedPathOrientation = ''; state.graph.classList.remove('is-focused-path'); } const focused = state.focusCameraKey && graphItems().find((item) => graphItemKey(item) === state.focusCameraKey); if (focused) focusGraphItem(focused); else scheduleTransform(); }); }
  function applyTransform() { if (!state.stage) return; state.stage.setAttribute('transform', `translate(${state.x} ${state.y}) scale(${state.scale})`); const percent = Math.round(state.scale * 100); const mode = state.scale < .7 ? 'Overview' : state.scale > 1.45 ? 'Detail' : 'Balanced'; state.graph.classList.remove('tl-scale-overview', 'tl-scale-balanced', 'tl-scale-detail'); state.graph.classList.add(`tl-scale-${mode.toLowerCase()}`); state.zoomLabel.textContent = `${percent}%`; state.zoomLabel.setAttribute('aria-label', `${mode} scale, ${percent} zoom`); state.zoomLabel.title = `${mode} scale`; updateMinimapViewport(); }
  function toggleFullscreen() { state.fullscreenActive = !state.fullscreenActive; state.root.classList.toggle('tl-fullscreen', state.fullscreenActive); state.fullscreenButton.setAttribute('aria-pressed', String(state.fullscreenActive)); state.fullscreenButton.textContent = state.fullscreenActive ? 'Exit full screen' : 'Full screen'; if (state.fullscreenActive) { state.fullscreenFocus = document.activeElement; state.bodyOverflow = document.body.style.overflow; document.body.style.overflow = 'hidden'; state.graph.focus(); } else { document.body.style.overflow = state.bodyOverflow || ''; if (state.fullscreenFocus && state.fullscreenFocus.isConnected) state.fullscreenFocus.focus(); state.fullscreenFocus = null; } scheduleResponsivePathLayout(); }
  function exportDataset() { if (!state.dataset) return; const blob = new Blob([JSON.stringify(state.dataset, null, 2)], { type: 'application/json' }); const url = URL.createObjectURL(blob); const link = el('a'); link.href = url; link.download = 'graphlayer-synthetic-topology-lab.json'; link.click(); setTimeout(() => URL.revokeObjectURL(url), 0); }
  function exportCurrentView() {
    const exporter = window.GraphLayerGraphExport;
    if (!state.projection || state.busy || !exporter || typeof exporter.download !== 'function') return;
    const viewLabel = ({ l3: 'Layer 3 overview', zone: 'Zone overview', l2: 'Host and Layer 2 detail' })[state.view] || 'Network map';
    const filters = [['Network', state.network], ['Zone', state.zone], ['VRF', state.vrf], ['VDOM', state.vdom], ['Search', state.search]].filter(([, value]) => value).map(([name, value]) => `${name}: ${value}`);
    const description = [viewLabel, filters.length ? `Applied filters: ${filters.join('; ')}` : 'Applied filters: none', str(state.caption.textContent), 'Synthetic configuration-derived view only; this map does not establish live reachability, traffic, or runtime state.'].join('\n');
    try { exporter.download(state.drawing, { filename: 'graphlayer-network-map.svg', title: 'GraphLayer network map', description }); }
    catch (_) { state.caption.textContent = 'Network map SVG export failed. The current view is unchanged.'; }
  }
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
