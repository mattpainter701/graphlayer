(() => {
  'use strict';

  const state = { event: null, identity: '', selected: '', root: null, svg: null, list: null, stage: null, inspector: null, zoomOutput: null, scale: 1, x: 0, y: 0, base: null, pointers: new Map(), dragging: false, moved: false };
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
    const controls = node('div', 'topology-toolbar-actions'); controls.append(fit, zoomOut, zoom, zoomIn, focus, forward.wrap, returned.wrap);
    toolbar.append(searchWrap, controls);
    const deviceList = node('div', 'topology-device-list'); deviceList.id = 'topology-device-list'; deviceList.setAttribute('role', 'group'); deviceList.setAttribute('aria-label', 'Device search results');
    const body = node('div', 'topology-workbench-body'); const stage = node('div', 'topology-stage'); stage.id = 'topology-stage'; stage.tabIndex = 0; stage.setAttribute('role', 'region'); stage.setAttribute('aria-label', 'Interactive network topology. Use the device list to select a device.');
    const caption = node('div', 'topology-stage-caption', 'Declared topology draft · analysis not run · runtime observations unavailable'); caption.id = 'topology-caption'; caption.setAttribute('aria-live', 'polite');
    const canvas = node('div', 'topology-canvas'); canvas.append(state.svg); stage.append(canvas, caption);
    const inspector = node('aside', 'topology-inspector'); inspector.id = 'topology-inspector'; inspector.setAttribute('aria-label', 'Selected device details'); inspector.append(node('div', 'topology-inspector-empty', 'Select a device to inspect its saved configuration and selected path context.'));
    body.append(stage, inspector);
    const help = node('p', 'topology-workbench-note', 'Declared links come from the supplied topology. Forward and return overlays show configured route analysis; live session and traffic observations are unavailable.');
    state.list.replaceChildren();
    state.root.append(toolbar, deviceList, body, help);
    state.deviceList = deviceList; state.stage = stage; state.inspector = inspector; state.zoomOutput = zoom; state.search = search; state.caption = caption; state.forwardToggle = forward.input; state.returnToggle = returned.input;
    const applyZoom = () => { if (!state.base) return; const width = state.base.width / state.scale; const height = state.base.height / state.scale; state.svg.setAttribute('viewBox', `${state.x} ${state.y} ${width} ${height}`); state.zoomOutput.value = `${Math.round(state.scale * 100)}%`; state.zoomOutput.textContent = `${Math.round(state.scale * 100)}%`; };
    state.applyZoom = applyZoom;
    const svgPoint = (clientX, clientY) => { const matrix = state.svg.getScreenCTM(); if (!matrix) return null; const point = state.svg.createSVGPoint(); point.x = clientX; point.y = clientY; return point.matrixTransform(matrix.inverse()); };
    const setScale = (value, clientX, clientY) => { const next = Math.max(.25, Math.min(4, value)); const anchor = clientX === undefined ? null : svgPoint(clientX, clientY); if (anchor && state.base) { const px = (anchor.x - state.x) / (state.base.width / state.scale); const py = (anchor.y - state.y) / (state.base.height / state.scale); state.scale = next; state.x = anchor.x - px * state.base.width / next; state.y = anchor.y - py * state.base.height / next; } else state.scale = next; applyZoom(); };
    state.fit = () => { if (!state.base) return; state.scale = 1; state.x = state.base.x; state.y = state.base.y; applyZoom(); };
    fit.addEventListener('click', () => state.fit()); zoomIn.addEventListener('click', () => setScale(state.scale * 1.2)); zoomOut.addEventListener('click', () => setScale(state.scale / 1.2));
    focus.addEventListener('click', () => { if (state.selected) selectNode(state.selected, true); else state.search.focus(); });
    search.addEventListener('input', renderDeviceList);
    forward.input.addEventListener('change', () => { state.svg.classList.toggle('topology-hide-forward', !forward.input.checked); });
    returned.input.addEventListener('change', () => { state.svg.classList.toggle('topology-hide-return', !returned.input.checked); });
    stage.addEventListener('wheel', (event) => { event.preventDefault(); setScale(state.scale * (event.deltaY < 0 ? 1.12 : 1 / 1.12), event.clientX, event.clientY); }, { passive: false });
    stage.addEventListener('pointerdown', (event) => { if (event.button !== 0 || event.target.closest('.topology-node')) return; state.dragging = true; state.moved = false; state.pointers.set(event.pointerId, { x: event.clientX, y: event.clientY }); stage.setPointerCapture(event.pointerId); stage.classList.add('is-panning'); });
    stage.addEventListener('pointermove', (event) => { if (!state.dragging || !state.pointers.has(event.pointerId)) return; const previous = state.pointers.get(event.pointerId); const oldPoint = svgPoint(previous.x, previous.y); const nextPoint = svgPoint(event.clientX, event.clientY); state.pointers.set(event.pointerId, { x: event.clientX, y: event.clientY }); if (Math.abs(event.clientX - previous.x) + Math.abs(event.clientY - previous.y) > 1) state.moved = true; if (state.base && oldPoint && nextPoint) { state.x -= nextPoint.x - oldPoint.x; state.y -= nextPoint.y - oldPoint.y; applyZoom(); } });
    const endPan = (event) => { state.pointers.delete(event.pointerId); if (!state.pointers.size) { state.dragging = false; stage.classList.remove('is-panning'); setTimeout(() => { state.moved = false; }, 0); } };
    stage.addEventListener('pointerup', endPan); stage.addEventListener('pointercancel', endPan);
    state.svg.addEventListener('click', (event) => { if (state.moved) { event.preventDefault(); event.stopImmediatePropagation(); return; } const target = event.target.closest('.topology-node'); if (!target) return; event.preventDefault(); event.stopImmediatePropagation(); selectNode(target.dataset.nodeId); }, true);
    state.svg.addEventListener('keydown', (event) => { const target = event.target.closest('.topology-node'); if (!target || !['Enter', ' '].includes(event.key)) return; event.preventDefault(); event.stopImmediatePropagation(); selectNode(target.dataset.nodeId); }, true);
    window.addEventListener('resize', () => { if (state.base) applyZoom(); });
    state.svg.classList.toggle('topology-hide-forward', !forward.input.checked); state.svg.classList.toggle('topology-hide-return', !returned.input.checked);
  }

  function getRequest() { const e = state.event || {}; return e.request || e.exploration && e.exploration.request || {}; }
  function getNodes() { return getRequest().nodes || []; }
  function getBranch() { const e = state.event || {}; const branches = e.exploration && e.exploration.result && e.exploration.result.branches || []; return branches[e.branchIndex] || null; }
  function getReturn() { if (state.event && state.event.mode === 'trace') return getTraceResult() && getTraceResult().return || null; const branch = getBranch(); return branch && branch.returns && branch.returns[state.event.returnIndex] || null; }
  function getTraceResult() { return state.event && state.event.investigation && state.event.investigation.result || null; }
  function getHops(direction) { if (state.event && state.event.mode === 'trace') { const side = getTraceResult() && getTraceResult()[direction]; return side && side.hops || []; } if (direction === 'forward') return getBranch() && getBranch().forward && getBranch().forward.hops || []; return getReturn() && getReturn().direction && getReturn().direction.hops || []; }
  function getSnapshot(id) { const entry = (state.event && state.event.snapshots || []).find((item) => item && item.node_id === id); return entry && entry.snapshot || null; }
  function deviceSearchText(item) { const snapshot = getSnapshot(item.node_id); return [item.node_id, snapshot && snapshot.hostname, snapshot && snapshot.vendor, snapshot && snapshot.context].filter(Boolean).join(' ').toLocaleLowerCase(); }
  function sourceButton(snapshotId, ref, label, host) { if (!snapshotId || !ref) return; const button = node('button', 'topology-source-button', label || `Source ${String(ref).slice(0, 18)}`); button.type = 'button'; button.addEventListener('click', () => document.dispatchEvent(new CustomEvent('graphlayer:open-network-evidence', { detail: { snapshot_id: snapshotId, ref, trigger: button } }))); host.append(button); }
  function omittedNote(host, omitted, label) { if (omitted > 0) host.append(node('p', 'topology-omitted-note', `${omitted} additional ${label} omitted from this view.`)); }
  function boundedArray(host, values, limit, label, render) { const items = Array.isArray(values) ? values : []; items.slice(0, limit).forEach(render); omittedNote(host, Math.max(0, items.length - limit), label); }
  function evidenceRefs(snapshotId, refs, host, title) { if (!Array.isArray(refs) || !refs.length) return; const row = node('div', 'topology-source-row'); row.append(node('span', 'topology-meta-label', title)); refs.slice(0, 6).forEach((entry) => { const ref = typeof entry === 'object' && entry ? entry.ref || entry.id : entry; const sid = typeof entry === 'object' && entry && entry.snapshot_id || snapshotId; const owner = (state.event && state.event.snapshots || []).map((item) => item.snapshot).find((snapshot) => snapshot && snapshot.id === sid); const evidence = owner && (owner.evidence || []).find((item) => item.id === ref); const label = evidence ? `${String(evidence.kind || 'Source').replaceAll('_', ' ')} · lines ${evidence.start_line}–${evidence.end_line}` : 'Open source'; sourceButton(sid, ref, label, row); }); omittedNote(row, Math.max(0, refs.length - 6), 'source references'); if (row.childElementCount > 1) host.append(row); }
  function renderDeviceList() {
    if (!state.deviceList) return;
    const query = (state.search.value || '').trim().toLocaleLowerCase(); const nodes = getNodes().filter((item) => deviceSearchText(item).includes(query));
    state.deviceList.replaceChildren(); state.deviceList.setAttribute('aria-label', query ? 'Matching devices' : 'Devices');
    if (!nodes.length) state.deviceList.append(node('span', 'topology-no-results', query ? 'No matching devices' : 'No devices'));
    nodes.forEach((item) => { const snapshot = getSnapshot(item.node_id); const button = node('button', 'topology-device', item.node_id); button.type = 'button'; button.dataset.nodeId = item.node_id; button.setAttribute('aria-label', `${item.node_id}${snapshot && snapshot.hostname ? ` · ${snapshot.hostname}` : ''}${snapshot && snapshot.vendor ? ` · ${snapshot.vendor}` : ''}${snapshot && snapshot.context ? ` · ${snapshot.context}` : ''}`); button.setAttribute('aria-pressed', String(state.selected === item.node_id)); button.addEventListener('click', () => selectNode(item.node_id, true)); state.deviceList.append(button); });
    state.svg.querySelectorAll('.topology-node').forEach((item) => { const match = nodes.some((candidate) => candidate.node_id === item.dataset.nodeId); item.classList.toggle('topology-search-dim', Boolean(query && !match)); item.classList.toggle('topology-search-match', Boolean(query && match)); });
  }
  function renderInspector(nodeId) {
    const snapshot = getSnapshot(nodeId); const entry = (state.event && state.event.snapshots || []).find((item) => item && item.node_id === nodeId); const hops = [...getHops('forward'), ...getHops('return')].filter((hop) => hop.node_id === nodeId);
    const panel = node('div', 'topology-inspector-content'); const heading = node('div', 'topology-inspector-heading'); heading.append(node('div', 'topology-eyebrow', 'DEVICE INSPECTOR'), node('h4', '', nodeId)); panel.append(heading);
    const summary = node('dl', 'topology-summary'); const add = (label, value) => { summary.append(node('dt', '', label), node('dd', '', safeText(value))); };
    add('Vendor', snapshot && snapshot.vendor); add('Context', snapshot && snapshot.context); add('Hostname', snapshot && snapshot.hostname); add('Snapshot', snapshot && snapshot.id || entry && entry.snapshot_id);
    add('Sanitized configuration SHA-256', snapshot && snapshot.sanitized_sha256); panel.append(summary);
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
    panel.append(facts);
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
    routeContext.append(node('p', 'topology-runtime-unavailable', 'Runtime session state, packet captures, counters, and live forwarding observations are unavailable.')); panel.append(routeContext);
    state.inspector.replaceChildren(panel);
  }
  function selectNode(nodeId, focus) {
    if (!nodeId) return; state.selected = nodeId; state.svg.querySelectorAll('.topology-node').forEach((item) => { const selected = item.dataset.nodeId === nodeId; item.classList.toggle('topology-selected', selected); item.setAttribute('aria-pressed', String(selected)); });
    state.deviceList.querySelectorAll('.topology-device').forEach((button) => button.setAttribute('aria-pressed', String(button.dataset.nodeId === nodeId)));
    renderInspector(nodeId); if (focus) focusNode(nodeId);
  }
  function focusNode(nodeId) {
    const target = [...state.svg.querySelectorAll('.topology-node')].find((item) => item.dataset.nodeId === nodeId); const circle = target && target.querySelector('circle'); if (!target || !circle || !state.base) return;
    const centerX = Number(circle.getAttribute('cx')); const centerY = Number(circle.getAttribute('cy')); state.scale = Math.max(1.5, Math.min(4, state.scale)); const width = state.base.width / state.scale; const height = state.base.height / state.scale; state.x = centerX - width / 2; state.y = centerY - height / 2; state.applyZoom(); target.focus({ preventScroll: true });
  }
  function update(event) {
    setup(); if (!state.root || !event || !event.detail) return;
    const request = event.detail.request || event.detail.exploration && event.detail.exploration.request || {};
    const nextIdentity = event.detail.mode === 'draft' ? `draft:${JSON.stringify(request)}` : event.detail.mode === 'trace' ? `trace:${event.detail.investigation && event.detail.investigation.id || JSON.stringify(request)}` : `result:${event.detail.exploration && event.detail.exploration.id || JSON.stringify(request)}`;
    state.event = event.detail;
    const vb = (state.svg.getAttribute('viewBox') || '0 0 720 250').trim().split(/[ ,]+/).map(Number); if (vb.length === 4 && vb.every(Number.isFinite)) { state.base = { x: vb[0], y: vb[1], width: vb[2], height: vb[3] }; }
    const changedIdentity = state.identity !== nextIdentity; state.identity = nextIdentity;
    if (changedIdentity) { state.selected = ''; state.fit(); } else state.applyZoom();
    state.svg.classList.toggle('topology-hide-forward', !state.forwardToggle.checked); state.svg.classList.toggle('topology-hide-return', !state.returnToggle.checked);
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
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', setup, { once: true }); else setup();
})();
