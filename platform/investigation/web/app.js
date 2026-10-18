(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const state = { csrf: '', snapshots: [], snapshot: null, investigation: null, pathInvestigation: null, networkInvestigation: null, networkExploration: null, networkSnapshots: new Map(), busy: false, scopeRequest: 0, toastTimer: 0, selectedObjectID: '', selectedRuleID: '', inventoryPage: 0, objectPage: 0, evidenceTrigger: null };
  const inventoryPageSize = 100;
  const objectPageSize = 50;
  const apiRoot = '/api';

  function el(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = String(text);
    return node;
  }

  async function request(path, options = {}) {
    const headers = new Headers(options.headers || {});
    if (options.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
    if (options.method && options.method.toUpperCase() !== 'GET') {
      headers.set('X-CSRF-Token', state.csrf);
      headers.set('Origin', window.location.origin);
    }
    let response;
    try {
      response = await fetch(apiRoot + path, { ...options, headers, credentials: 'same-origin', cache: 'no-store' });
    } catch (_) {
      throw new Error('The local service could not be reached. Check that it is running and try again.');
    }
    let body;
    try { body = await response.json(); } catch (_) { body = null; }
    if (!response.ok) {
      const code = body && body.error && body.error.code;
      const messages = {
        invalid_flow: 'Use concrete IP addresses and valid protocol/port values for this investigation.',
        invalid_path: 'Use a supported ingress interface and a concrete IPv4 flow with valid protocol/port values.',
        path_unavailable: 'This snapshot does not include a usable routing and NAT model. Reimport the configuration after checking its supported path settings.',
        invalid_network_path: 'Check that each device has a unique ID and snapshot, that every supplied link and endpoint uses a modeled interface, and that the flow is a concrete IPv4 tuple.',
        path_too_large: 'The path investigation exceeds the local artifact budget. No partial result was saved.',
        invalid_import: 'Provide a supported firewall configuration and a display name of at most 128 characters.',
        import_failed: 'The configuration could not be imported. Earlier snapshots are unchanged. Use ASA running-config, PAN-OS set-format or FortiOS single-VDOM CLI configuration.',
        config_too_large: 'The configuration exceeds the 2 MiB local import limit.',
        input_too_large: 'The request exceeds the local import size limit.',
        snapshot_too_large: 'The expanded snapshot exceeds the 8 MiB artifact budget. Earlier snapshots are unchanged.',
        investigation_too_large: 'The investigation trace exceeds the 8 MiB artifact budget. No partial result was saved.',
        import_busy: 'An import is already running. Wait for it to finish, then retry.',
        workspace_full: 'The local workspace has reached its snapshot or investigation limit. Select a new workspace directory to continue.',
        workspace_write_failed: 'The local workspace could not save the request. Earlier records are unchanged. Check directory access and available disk space.',
        chain_required: 'Select an attached ACL chain before evaluating.',
        scope_not_found: 'That snapshot or chain is no longer available. Refresh the workspace.',
        snapshot_not_found: 'That snapshot is no longer available. Refresh the workspace.',
        investigation_not_found: 'That investigation is no longer available. Refresh the workspace.',
        not_found: 'That snapshot or investigation is no longer available. Refresh the workspace.',
        session_required: 'Reload the page to initialize the local session.',
        request_verification: 'Request verification failed. Reload the page and try again.',
        local_access: 'Open this app using its exact loopback URL.'
      };
      throw new Error(messages[code] || `The request could not be completed (${response.status}).`);
    }
    return body || {};
  }

  function showToast(message, kind = '') {
    const toast = $('toast');
    toast.textContent = message;
    toast.className = `toast visible ${kind}`;
    window.clearTimeout(state.toastTimer);
    state.toastTimer = window.setTimeout(() => { toast.className = 'toast'; }, 4000);
  }

  function setBusy(button, busy, label) {
    state.busy = busy;
    if (!button.dataset.idleText) {
      const text = Array.from(button.querySelectorAll('span')).find((node) => node.textContent.trim());
      button.dataset.idleText = text ? text.textContent : '';
    }
    button.classList.toggle('is-loading', busy);
    button.disabled = busy;
    const words = button.querySelectorAll('span');
    const text = Array.from(words).find((node) => node.textContent.trim());
    if (text) text.textContent = busy ? label : button.dataset.idleText;
    refreshControls();
  }

  function summaryName(snapshot) {
    return [snapshot.hostname || snapshot.name || 'Firewall', vendorName(snapshot.vendor), snapshot.context || 'default', snapshot.created_at ? new Date(snapshot.created_at).toLocaleString() : snapshot.id].filter(Boolean).join(' · ');
  }

  function vendorName(vendor) { return ({ 'cisco-asa': 'Cisco ASA', 'paloalto-panos': 'Palo Alto PAN-OS', 'fortinet-fortigate': 'FortiGate FortiOS' })[vendor] || vendor || 'Firewall'; }
  function supportedPath(config) { return Boolean(config && ['asa-path-v1','panos-path-v1','fortios-path-v1'].includes(config.version)); }

  function hashLabel(value) {
    if (!value) return 'Hash unavailable';
    return `${value.slice(0, 12)}…${value.slice(-6)}`;
  }

  function hashChip(label, value) {
    if (!value) return el('span', 'hash-chip', `${label} · unavailable`);
    const chip = el('button', 'hash-chip hash-copy', `${label}  ${hashLabel(value)}`);
    chip.type = 'button';
    chip.title = `${value} — SHA-256 identifies these uploaded bytes; it does not authenticate their origin or device identity. Click to copy.`;
    chip.setAttribute('aria-label', `Copy ${label} SHA-256 fingerprint ${value}`);
    chip.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(value);
        showToast(`${label} fingerprint copied.`);
      } catch (_) {
        chip.title = `${value} — select and copy this fingerprint. It does not authenticate origin or device identity.`;
        showToast(`Full fingerprint: ${value}`);
      }
    });
    return chip;
  }

  function updateScope() {
    const s = state.snapshot;
    $('scope-snapshot').textContent = s ? (s.name || s.hostname || s.id) : 'No snapshot selected';
    $('scope-device').textContent = s ? `${s.hostname || 'ASA device'} · ${s.context || 'default'}` : '—';
    $('scope-hash').textContent = s ? hashLabel(s.artifact_sha256) : 'Waiting for import';
    $('integrity-dot').classList.toggle('ready', Boolean(s));
    $('scope-hash').title = s && s.artifact_sha256
      ? `${s.artifact_sha256} — SHA-256 identifies the uploaded configuration bytes; it does not authenticate origin or device identity.`
      : '';
    $('scope-snapshot').title = s ? s.id : '';
    $('scope-device').title = s ? `${s.vendor || 'Cisco ASA'} · ${s.parser_version || 'parser version unavailable'}` : '';
  }

  function refreshControls() {
    const hasSnapshot = Boolean(state.snapshot);
    const configReady = $('config-input').value.trim().length > 0;
    $('import-button').disabled = state.busy || !configReady;
    $('snapshot-select').disabled = state.busy || state.snapshots.length === 0;
    $('chain-select').disabled = state.busy || !hasSnapshot || !state.snapshot.chains || state.snapshot.chains.length === 0;
    $('evaluate-button').disabled = state.busy || !hasSnapshot || !$('chain-select').value;
    ['config-input', 'file-input', 'load-sample', 'load-service-sample', 'flow-source', 'flow-destination', 'flow-protocol', 'refresh-button', 'refresh-history'].forEach((id) => { $(id).disabled = state.busy; });
    const usesPorts = $('flow-protocol').value === 'tcp' || $('flow-protocol').value === 'udp';
    $('flow-source-port').disabled = state.busy || !usesPorts;
    $('flow-destination-port').disabled = state.busy || !usesPorts;
    $('export-button').disabled = state.busy || !state.investigation;
    const pathAvailable = Boolean(hasSnapshot && supportedPath(state.snapshot.path));
    const pathReady = Boolean(pathAvailable && $('path-ingress').value);
    $('path-evaluate-button').disabled = state.busy || !pathReady;
    $('path-export-button').disabled = state.busy || !state.pathInvestigation;
    $('network-evaluate').disabled = state.busy || !networkFormValid();
    $('network-explore').disabled = state.busy || !networkFormValid();
    $('network-export').disabled = state.busy || !state.networkInvestigation;
    $('exploration-export').disabled = state.busy || !state.networkExploration;
    $('network-load-sample').disabled = state.busy;
    $('network-load-vendor-sample').disabled = state.busy || !state.csrf;
    $('network-load-exploration-sample').disabled = state.busy || !state.csrf;
    ['config-vendor','load-panos-sample','load-fortigate-sample'].forEach((id) => { $(id).disabled = state.busy; });
    $('network-add-node').disabled = state.busy || document.querySelectorAll('.network-node-row').length >= 16;
    $('network-add-link').disabled = state.busy || document.querySelectorAll('.network-link-row').length >= 32;
    document.querySelectorAll('.network-history-open,.network-remove-row').forEach((button) => { button.disabled = state.busy; });
    document.querySelectorAll('#network-path input,#network-path select').forEach((input) => { input.disabled = state.busy; });
    ['network-flow-source-port','network-flow-destination-port'].forEach((id) => { $(id).disabled = state.busy || !['tcp','udp'].includes($('network-flow-protocol').value); });
    $('refresh-network-history').disabled = state.busy;
    $('refresh-exploration-history').disabled = state.busy;
    ['path-ingress', 'path-source', 'path-destination', 'path-protocol'].forEach((id) => { const node = $(id); if (node) node.disabled = state.busy || !pathAvailable; });
    $('refresh-path-history').disabled = state.busy;
    const pathUsesPorts = $('path-protocol').value === 'tcp' || $('path-protocol').value === 'udp';
    ['path-source-port', 'path-destination-port'].forEach((id) => { $(id).disabled = state.busy || !pathAvailable || !pathUsesPorts; });
    document.querySelectorAll('.history-open').forEach((button) => { button.disabled = state.busy; });
  }

  function renderPathAvailability(snapshot) {
    const config = snapshot && snapshot.path;
    const valid = supportedPath(config);
    $('path-controls').hidden = !valid;
    $('path-result').hidden = true;
    $('path-unavailable').hidden = valid;
    if (!valid) {
      $('path-unavailable').textContent = snapshot
        ? 'Path analysis is unavailable for this snapshot. Reimport the configuration to build the routing and NAT model.'
        : 'Import a snapshot with routing and NAT configuration to investigate a path.';
      $('path-ingress').replaceChildren(new Option('Unavailable', ''));
      $('path-interface-meta').textContent = 'Ingress interface metadata is unavailable.';
      return;
    }
    const interfaces = Array.isArray(config.interfaces) ? config.interfaces : [];
    const choices = interfaces.filter((item) => item && item.name);
    const select = $('path-ingress');
    select.replaceChildren(new Option(choices.length ? 'Select ingress interface' : 'No configured interface', ''));
    choices.slice(0, 100).forEach((item) => {
      const status = `${item.resolved ? 'resolved' : 'unresolved'}${item.shutdown ? ' · shutdown' : ''}`;
      const label = `${item.name}${item.physical ? ` · ${item.physical}` : ''}${item.address ? ` · ${item.address}` : ''} · ${status}`;
      select.add(new Option(label, item.name));
    });
    const omittedCount = Math.max(0, choices.length - 100);
    $('path-interface-meta').textContent = choices.length
      ? `${choices.length} configured ingress interface${choices.length === 1 ? '' : 's'}${omittedCount ? ` · ${omittedCount} beyond the 100-interface display limit` : ''}. Unresolved or shut down choices return an indeterminate path.`
      : 'No configured ingress interface is available in this snapshot.';
    updatePathInterfaceMeta();
  }

  function updatePathInterfaceMeta() {
    const config = state.snapshot && state.snapshot.path;
    const item = config && (config.interfaces || []).find((candidate) => candidate.name === $('path-ingress').value);
    if (!item) return;
    const pieces = [item.name, item.physical, item.address, item.zone ? `zone ${item.zone}` : '', item.security_level === undefined ? '' : `security level ${item.security_level}`].filter(Boolean);
    $('path-interface-meta').textContent = `${pieces.join(' · ')} · ${item.resolved ? 'resolved' : 'unresolved'}${item.shutdown ? ' · shutdown' : ''}${config.routing_complete ? '' : ' · route coverage incomplete'}${config.nat_complete ? '' : ' · NAT coverage incomplete'}`;
  }

  function setCurrentSnapshot(snapshot, preserveChain = '') {
    state.snapshot = snapshot;
    state.investigation = null;
    state.pathInvestigation = null;
    state.selectedObjectID = '';
    state.selectedRuleID = '';
    state.inventoryPage = 0;
    state.objectPage = 0;
    updateScope();
    renderPathAvailability(snapshot);
    renderImportReport(snapshot);
    const chainSelect = $('chain-select');
    chainSelect.replaceChildren();
    const chains = Array.isArray(snapshot.chains) ? snapshot.chains : [];
    if (!chains.length) {
      chainSelect.add(new Option('No attached ACL chains found', ''));
      $('chain-meta').textContent = 'No evaluable chain in this snapshot';
      $('evaluate-button').disabled = true;
    } else {
      chainSelect.add(new Option('Select an attached chain', ''));
      chains.forEach((chain) => {
        const option = new Option(`${chain.name || chain.id} · ${chain.interface || 'interface unknown'} / ${chain.direction || 'direction unknown'}`, chain.id);
        chainSelect.add(option);
      });
      const preferred = chains.find((chain) => chain.id === preserveChain);
      if (preferred) chainSelect.value = preferred.id;
      updateChainMeta();
    }
    clearResult();
    renderInventory();
    renderObjects();
    refreshControls();
  }

  function activeRuleOrigins() {
    return new Map(((state.snapshot && state.snapshot.rule_origins) || []).map((origin) => [origin.rule_id, origin]));
  }

  function currentResultTraces() {
    const investigation = state.investigation;
    if (!investigation || !state.snapshot || investigation.snapshot_id !== state.snapshot.id || investigation.chain_id !== $('chain-select').value) return new Map();
    return new Map((((investigation.result || {}).rule_traces) || []).map((trace) => [trace.rule_id, trace]));
  }

  function resultTraceState(result) {
    if (!result) return 'none';
    if (result.engine_contract_version === 'graphlayer-policy-v2') return 'legacy';
    if (result.engine_contract_version === 'graphlayer-policy-v3' && !(result.considered_rules || []).length) return 'empty';
    return Array.isArray(result.rule_traces) ? 'available' : 'unavailable';
  }

  function navigateToObject(id) {
    state.selectedObjectID = id;
    $('object-search').value = '';
    const objects = (state.snapshot && state.snapshot.objects) || [];
    const index = objects.findIndex((object) => object.id === id);
    state.objectPage = index < 0 ? 0 : Math.floor(index / objectPageSize);
    renderObjects();
    $('objects').scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  function formatRulePorts(rule) {
    const protocol = rule.protocol || 'any';
    const src = rule.source_port ? `src ${rule.source_port}` : '';
    const dst = rule.destination_port ? `dst ${rule.destination_port}` : '';
    return [protocol, src, dst].filter(Boolean).join(' · ');
  }

  function ruleOriginLabel(rule, origins) {
    const origin = origins.get(rule.id);
    return origin ? `${origin.acl_name || 'ACL'} · line ${origin.line}` : 'Original ACE line unavailable';
  }

  function renderPredicateTrace(ruleID) {
    const panel = $('predicate-trace');
    panel.replaceChildren();
    panel.append(el('strong', '', 'Predicate explanation'));
    const investigation = state.investigation;
    const traces = currentResultTraces();
    const trace = traces.get(ruleID);
    if (!investigation) {
      panel.append(el('p', '', 'Evaluate this flow to see saved evaluator explanations for considered rules.'));
      return;
    }
    const traceState = resultTraceState(investigation.result);
    if (traceState === 'legacy') {
      panel.append(el('p', '', 'Predicate traces are unavailable for this graphlayer-policy-v2 legacy result.'));
      return;
    }
    if (traceState === 'empty') {
      panel.append(el('p', '', 'No enabled rules were considered for this result.'));
      return;
    }
    if (traceState !== 'available') {
      panel.append(el('p', '', 'Rule traces are unavailable in this saved result.'));
      return;
    }
    if (!trace) {
      panel.append(el('p', '', 'This rule has no trace in the saved result. It was not evaluated for this investigation.'));
      return;
    }
    panel.append(el('p', '', `Saved outcome: ${trace.outcome || 'unavailable'} · position ${trace.position ?? ruleID}.`));
    const list = el('ul', 'predicate-list');
    (trace.predicates || []).forEach((predicate) => {
      const item = el('li', `predicate-state predicate-${predicate.state || 'unknown'}`);
      item.append(el('strong', '', `${predicate.dimension || 'Predicate'} · ${predicate.state || 'state unavailable'}`));
      item.append(el('span', '', predicate.reason || 'No reason was saved.'));
      list.append(item);
    });
    if (!list.childElementCount) list.append(el('li', '', 'No predicate details were saved for this trace.'));
    panel.append(list);
    if (investigation.result.disposition === 'indeterminate' && trace.outcome === 'matched') panel.append(el('p', 'trace-boundary', 'A matching rule trace does not establish a permit because the overall chain result is indeterminate.'));
  }

  function renderRuleInspector(rule) {
    const panel = $('rule-inspector'); panel.replaceChildren();
    if (!rule) { panel.append(el('div', 'inspector-placeholder', 'Select a rule to inspect its normalized predicates and original ACE line.')); return; }
    const origins = activeRuleOrigins();
    const origin = origins.get(rule.id);
    const head = el('div', 'inspector-heading');
    head.append(el('div', '', `${rule.id} · position ${rule.position ?? 'unavailable'}`), el('span', `action-tag ${rule.action === 'deny' ? 'action-deny' : 'action-permit'}`, rule.action || 'Action unavailable'));
    panel.append(head);
    const fields = el('dl', 'rule-predicate-grid');
    [['Source', rule.source], ['Destination', rule.destination], ['Protocol', rule.protocol], ['Source port', rule.source_port], ['Destination port', rule.destination_port], ['State', rule.enabled ? 'Enabled' : 'Disabled']].forEach(([label, value]) => {
      fields.append(el('dt', '', label), el('dd', '', value || (label.includes('port') ? 'Any / unavailable' : 'Unavailable')));
    });
    panel.append(fields);
    const original = el('div', 'ace-source');
    original.append(el('strong', '', origin ? `${origin.acl_name || 'ACL'} · physical line ${origin.line}` : 'Original ACE source unavailable'));
    if (origin && Number.isInteger(origin.line)) {
      const source = ((state.snapshot && state.snapshot.source_lines) || []).find((line) => line.number === origin.line);
      if (source) original.append(el('pre', '', source.text));
      else original.append(el('p', '', 'The origin line is not present in this snapshot’s source lines.'));
      const openLine = el('button', 'open-evidence', `Open physical line ${origin.line} ↗`);
      openLine.type = 'button'; openLine.addEventListener('click', (event) => showEvidence({ kind: 'Original ACE', start_line: origin.line, end_line: origin.line, summary: `Original ACE at line ${origin.line}` }, event.currentTarget));
      original.append(openLine);
    }
    panel.append(original);
    const originEvidence = el('div', 'rule-dependencies'); originEvidence.append(el('strong', '', 'Rule evidence'));
    const evidenceItems = evidenceForRefs(rule.evidence_refs || []);
    if (!evidenceItems.length) originEvidence.append(el('p', '', rule.evidence_refs && rule.evidence_refs.length ? 'Evidence references are unavailable in this snapshot.' : 'No rule evidence references were recorded.'));
    else evidenceItems.forEach((item) => originEvidence.append(evidenceLink(item)));
    panel.append(originEvidence);
    const deps = el('div', 'rule-dependencies'); deps.append(el('strong', '', 'Named dependencies'));
    if (!origin) deps.append(el('p', '', 'Rule origin metadata is unavailable for this snapshot.'));
    else {
      const groups = [['Source objects', origin.source_object_ids], ['Destination objects', origin.destination_object_ids], ['Service objects', origin.service_object_ids]];
      groups.forEach(([title, ids]) => {
        const row = el('div', 'dependency-ref-row'); row.append(el('span', '', title));
        if (!(ids || []).length) row.append(el('span', 'muted-value', 'None recorded'));
        (ids || []).forEach((id) => { const object = ((state.snapshot.objects) || []).find((item) => item.id === id); const button = el('button', 'object-ref-link', object ? `${object.name} · ${object.kind}` : `Missing object · ${id}`); button.type = 'button'; button.addEventListener('click', () => navigateToObject(id)); row.append(button); });
        deps.append(row);
      });
    }
    panel.append(deps);
  }

  function renderInventory() {
    const tbody = $('inventory-body'); if (!tbody) return;
    tbody.replaceChildren();
    const chain = selectedChain();
    const snapshot = state.snapshot;
    const all = chain && Array.isArray(chain.rules) ? chain.rules.slice().sort((a, b) => (a.position ?? Number.MAX_SAFE_INTEGER) - (b.position ?? Number.MAX_SAFE_INTEGER)) : [];
    $('inventory-scope').textContent = snapshot && chain ? `${snapshot.name || snapshot.hostname || snapshot.id} · ${chain.name || chain.id} · ${snapshot.id}` : 'No snapshot or chain selected';
    const query = $('inventory-search').value.trim().toLocaleLowerCase();
    const action = $('inventory-action').value;
    const status = $('inventory-status').value;
    const unresolvedOnly = $('inventory-resolution').value === 'unresolved';
    const origins = activeRuleOrigins();
    const objectNames = new Map(((snapshot && snapshot.objects) || []).map((object) => [object.id, object.name || object.id]));
    const visible = all.filter((rule) => {
      const origin = origins.get(rule.id);
      const dependencyIDs = origin ? ['source_object_ids', 'destination_object_ids', 'service_object_ids'].flatMap((key) => origin[key] || []) : [];
      const dependencyTerms = dependencyIDs.flatMap((id) => [id, objectNames.get(id) || '']);
      const searchable = [rule.id, rule.source, rule.destination, rule.protocol, rule.source_port, rule.destination_port, origin && origin.acl_name, ...dependencyTerms, ...(rule.unsupported || [])].join(' ').toLocaleLowerCase();
      const unresolvedReferences = origin && ['source_object_ids', 'destination_object_ids', 'service_object_ids'].some((key) => (origin[key] || []).some((id) => {
        const referenced = (snapshot.objects || []).find((object) => object.id === id);
        return !referenced || !referenced.resolved;
      }));
      const unresolved = (rule.unsupported || []).length > 0 || Boolean(unresolvedReferences);
      return (!query || searchable.includes(query)) && (action === 'all' || rule.action === action) && (status === 'all' || (status === 'enabled') === Boolean(rule.enabled)) && (!unresolvedOnly || unresolved);
    });
    const pageCount = Math.max(1, Math.ceil(visible.length / inventoryPageSize));
    state.inventoryPage = Math.min(state.inventoryPage, pageCount - 1);
    const start = state.inventoryPage * inventoryPageSize;
    const pageRules = visible.slice(start, start + inventoryPageSize);
    $('inventory-count').textContent = `${visible.length} filtered · ${all.length} total · page ${state.inventoryPage + 1} of ${pageCount} · ${chain ? (chain.complete ? 'parser reports supplied ACE coverage complete' : 'partial chain') : 'no chain selected'}`;
    $('inventory-page').textContent = `Page ${state.inventoryPage + 1} of ${pageCount}`;
    $('inventory-prev').disabled = state.inventoryPage === 0;
    $('inventory-next').disabled = state.inventoryPage >= pageCount - 1 || visible.length === 0;
    $('inventory-empty').hidden = visible.length > 0;
    $('inventory-empty').textContent = !snapshot ? 'Import a snapshot to browse its policy inventory.' : !chain ? 'Select an attached ACL chain in the investigation flow panel.' : all.length ? 'No rules match these filters.' : 'This chain contains no normalized rules.';
    const traces = currentResultTraces();
    if (!visible.some((rule) => rule.id === state.selectedRuleID)) state.selectedRuleID = '';
    pageRules.forEach((rule) => {
      const row = el('tr', 'inventory-row'); row.tabIndex = 0; row.dataset.ruleId = rule.id;
      const trace = traces.get(rule.id);
      const traceState = resultTraceState(state.investigation && state.investigation.result);
      const traceLabel = trace ? `Saved trace · ${trace.outcome}` : traceState === 'legacy' ? 'Legacy result · no trace' : traceState === 'empty' ? 'No enabled rules considered' : 'No saved trace';
      const values = [rule.position ?? '—', `${rule.id}\n${ruleOriginLabel(rule, origins)}`, rule.action || '—', !rule.enabled ? 'Disabled' : traceLabel, rule.source || '—', rule.destination || '—', formatRulePorts(rule) || 'any'];
      values.forEach((value, index) => { const cell = el('td', index === 1 || index > 3 ? 'inventory-predicate' : '', value); row.append(cell); });
      row.setAttribute('aria-selected', String(rule.id === state.selectedRuleID));
      const select = () => { state.selectedRuleID = rule.id; tbody.querySelectorAll('.inventory-row[aria-selected="true"]').forEach((old) => old.setAttribute('aria-selected', 'false')); row.setAttribute('aria-selected', 'true'); renderRuleInspector(rule); };
      row.setAttribute('aria-label', `Rule ${rule.id}, position ${rule.position ?? 'unknown'}, ${rule.action || 'action unknown'}, ${rule.enabled ? 'enabled' : 'disabled'}`);
      row.addEventListener('click', select); row.addEventListener('keydown', (event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); select(); } });
      tbody.append(row);
    });
    renderRuleInspector(visible.find((rule) => rule.id === state.selectedRuleID) || null);
  }

  function renderObjects() {
    const list = $('object-list'); if (!list) return;
    list.replaceChildren();
    const snapshot = state.snapshot;
    $('object-scope').textContent = snapshot ? `${snapshot.name || snapshot.hostname || snapshot.id} · ${snapshot.id}` : 'No snapshot selected';
    if (!snapshot || (!Array.isArray(snapshot.objects) && !['asa-config-v2', 'asa-config-v3','panos-config-v1','fortios-config-v1'].includes(snapshot.parser_version))) {
      list.append(el('div', 'inventory-empty', snapshot ? 'Named object inventory is unavailable in this older snapshot.' : 'Import a snapshot to browse named objects.'));
      $('object-count').textContent = '0 filtered · unavailable total · page 1 of 1';
      $('object-page').textContent = 'Page 1 of 1';
      $('object-prev').disabled = true; $('object-next').disabled = true;
      $('object-inspector').replaceChildren(el('div', 'inspector-placeholder', snapshot ? 'This snapshot does not include Config.objects.' : 'Select a snapshot to inspect its named dependencies.'));
      return;
    }
    const snapshotObjects = Array.isArray(snapshot.objects) ? snapshot.objects : [];
    const query = $('object-search').value.trim().toLocaleLowerCase();
    const filteredObjects = snapshotObjects.filter((object) => [object.name, object.kind, object.id].join(' ').toLocaleLowerCase().includes(query));
    const objectPageCount = Math.max(1, Math.ceil(filteredObjects.length / objectPageSize));
    state.objectPage = Math.min(state.objectPage, objectPageCount - 1);
    const objects = filteredObjects.slice(state.objectPage * objectPageSize, (state.objectPage + 1) * objectPageSize);
    $('object-count').textContent = `${filteredObjects.length} filtered · ${snapshotObjects.length} total · page ${state.objectPage + 1} of ${objectPageCount}`;
    $('object-page').textContent = `Page ${state.objectPage + 1} of ${objectPageCount}`;
    $('object-prev').disabled = state.objectPage === 0;
    $('object-next').disabled = state.objectPage >= objectPageCount - 1 || filteredObjects.length === 0;
    if (!filteredObjects.length) list.append(el('div', 'inventory-empty', snapshotObjects.length ? 'No objects match this search.' : 'No named objects were recorded in this snapshot.'));
    objects.forEach((object) => {
      const button = el('button', `object-option${object.id === state.selectedObjectID ? ' selected' : ''}`);
      button.type = 'button'; button.setAttribute('role', 'option'); button.setAttribute('aria-selected', String(object.id === state.selectedObjectID)); button.append(el('strong', '', object.name || object.id), el('span', '', `${object.kind} · ${object.resolved ? 'resolved' : 'unresolved'}`)); button.title = object.id;
      button.addEventListener('click', () => { state.selectedObjectID = object.id; renderObjects(); });
      list.append(button);
    });
    const object = snapshotObjects.find((item) => item.id === state.selectedObjectID);
    const inspector = $('object-inspector'); inspector.replaceChildren();
    if (!object) {
      if (state.selectedObjectID) inspector.append(el('div', 'missing-object', `Missing reference · ${state.selectedObjectID}. No same-name object was substituted.`));
      else inspector.append(el('div', 'inspector-placeholder', 'Select an object to inspect its members and dependencies.'));
      return;
    }
    inspector.append(el('h3', '', object.name || object.id), el('p', 'object-kind', `${object.kind} · ${object.resolved ? 'Resolved' : 'Unresolved'}`), el('code', 'object-id', object.id));
    if (object.protocol) inspector.append(el('p', '', `Protocol: ${object.protocol}`));
    const warnings = object.warnings || [];
    if (warnings.length) { const warningList = el('ul', 'object-warnings'); warnings.forEach((warning) => warningList.append(el('li', '', warning.line ? `Line ${warning.line}: ${warning.message || warning.code}` : warning.message || warning.code || 'Unspecified warning'))); inspector.append(warningList); }
    const tree = el('ul', 'dependency-tree');
    const byID = new Map(snapshotObjects.map((item) => [item.id, item]));
    const dependencyNodeBudget = 500;
    let renderedDependencyNodes = 0;
    let dependencyTreeTruncated = false;
    const expand = (current, parent, stack, depth) => {
      if (depth >= 16) { parent.append(el('li', 'dependency-warning', 'Maximum dependency depth reached; select the linked object to inspect it.')); return; }
      for (const member of (current.members || [])) {
        if (renderedDependencyNodes >= dependencyNodeBudget) { dependencyTreeTruncated = true; break; }
        renderedDependencyNodes += 1;
        const item = el('li', `dependency-member dependency-${member.kind || 'unknown'}`);
        const summary = [member.kind || 'member', member.value, member.protocol && `protocol ${member.protocol}`, member.source_port && `source port ${member.source_port}`, member.destination_port && `destination port ${member.destination_port}`].filter(Boolean).join(' · ');
        item.append(el('span', '', summary || 'Member details unavailable'));
        if (member.object_id) {
          const target = byID.get(member.object_id);
          item.append(el('code', 'dependency-id', member.object_id));
          if (!target) item.append(el('span', 'dependency-warning', 'Missing reference; no same-name rebinding was attempted.'));
          else if (stack.has(target.id)) item.append(el('span', 'dependency-warning', 'Cycle detected; expansion stopped.'));
          else { const nextStack = new Set(stack); nextStack.add(target.id); const targetLink = el('button', 'dependency-target', `${target.name} · ${target.kind} · ${target.resolved ? 'resolved' : 'unresolved'}`); targetLink.type = 'button'; targetLink.addEventListener('click', () => navigateToObject(target.id)); item.append(targetLink); if (target.members && target.members.length) { const nested = el('ul', 'dependency-tree nested'); expand(target, nested, nextStack, depth + 1); item.append(nested); } }
        }
        (member.evidence_refs || []).forEach((ref) => item.append(renderInventoryEvidence(ref)));
        parent.append(item);
      }
    };
    if (!object.members || !object.members.length) tree.append(el('li', 'muted-value', 'No members recorded.'));
    else expand(object, tree, new Set([object.id]), 0);
    if (dependencyTreeTruncated) tree.append(el('li', 'dependency-warning', 'More dependencies are available; select an object to inspect them.'));
    inspector.append(el('h4', '', 'Dependency tree'), tree);
    (object.evidence_refs || []).forEach((ref) => inspector.append(renderInventoryEvidence(ref)));
  }

  function renderInventoryEvidence(ref) {
    const evidence = ((state.snapshot && state.snapshot.evidence) || []).find((item) => item.id === ref);
    if (!evidence) return el('span', 'ref-unresolved', `Evidence unavailable · ${ref}`);
    return evidenceLink(evidence);
  }

  function renderImportReport(snapshot) {
    const panel = $('import-report');
    panel.replaceChildren();
    panel.className = 'import-report';
    const top = el('div', 'report-top');
    const mark = el('span', 'report-status-mark', '✓');
    const summary = el('div', 'report-summary');
    summary.append(el('strong', '', 'Configuration parsed'), el('span', '', `${snapshot.vendor || 'Cisco ASA'} · ${snapshot.parser_version || 'parser version unavailable'}`));
    top.append(mark, summary);
    const warnings = Array.isArray(snapshot.warnings) ? snapshot.warnings : [];
    const chainWarnings = (snapshot.chains || []).flatMap((c) => c.warnings || []);
    const allWarnings = [...warnings, ...chainWarnings];
    const stats = el('div', 'report-stats');
    [[snapshot.chain_count ?? (snapshot.chains || []).length, 'ACL chains'], [snapshot.rule_count ?? (snapshot.chains || []).reduce((n, c) => n + (c.rules || []).length, 0), 'normalized rules'], [allWarnings.length, 'warnings']].forEach(([value, label]) => {
      const item = el('div', 'report-stat'); item.append(el('strong', '', value), el('span', '', label)); stats.append(item);
    });
    panel.append(top, stats);
    const chips = el('div', 'hash-row');
    chips.append(hashChip('Artifact SHA-256', snapshot.artifact_sha256));
    if (snapshot.sanitized_sha256) chips.append(hashChip('Sanitized SHA-256', snapshot.sanitized_sha256));
    (snapshot.capabilities || []).forEach((capability) => chips.append(el('span', 'capability-chip', capability)));
    panel.append(chips);
    if (allWarnings.length) {
      const warningBox = el('div', 'warning-box');
      warningBox.append(el('span', 'warning-icon', '!'));
      const content = el('div', 'warning-content');
      content.append(el('strong', '', `${allWarnings.length} parser warning${allWarnings.length === 1 ? '' : 's'} · review before relying on this result`));
      const list = el('ul', 'warning-list');
      allWarnings.slice(0, 5).forEach((warning) => {
        const li = el('li');
        const message = warning.message || warning.code || 'Unspecified parser warning';
        li.textContent = warning.line ? `Line ${warning.line}: ${message}` : message;
        if (warning.line) li.dataset.line = String(warning.line);
        list.append(li);
      });
      if (allWarnings.length > 5) list.append(el('li', '', `${allWarnings.length - 5} additional warnings`));
      content.append(list); warningBox.append(content); panel.append(warningBox);
    } else {
      const ready = el('div', 'complete-note'); ready.append(el('span', '', '✓'), el('span', '', 'No parser warnings reported for this snapshot.')); panel.append(ready);
    }
  }

  function updateChainMeta() {
    const chain = selectedChain();
    const meta = $('chain-meta');
    meta.replaceChildren();
    const dot = el('span', `chain-dot${chain && chain.complete ? ' complete' : ''}`);
    const text = chain ? `${chain.complete ? 'Parsed chain' : 'Partial chain'} · ${(chain.rules || []).length} rules` : 'Select a chain';
    meta.append(dot, el('span', '', text));
    meta.classList.toggle('incomplete', Boolean(chain && !chain.complete));
    meta.title = chain
      ? `${chain.complete ? 'Parser reports supplied ACE coverage complete.' : 'Parser reports incomplete coverage among supplied ACEs.'} This flag does not establish complete device policy or runtime behavior.`
      : '';
    $('evaluate-button').disabled = state.busy || !chain;
  }

  function selectedChain() {
    if (!state.snapshot) return null;
    return (state.snapshot.chains || []).find((chain) => chain.id === $('chain-select').value) || null;
  }

  function clearResult() {
    state.investigation = null;
    $('result-empty').hidden = false;
    $('result-content').hidden = true;
    $('export-button').disabled = true;
    $('result-time').textContent = '';
    $('source-lines').hidden = true;
    $('evidence-cards').replaceChildren(el('div', 'evidence-placeholder', 'Select a rule in the trace to inspect its source.'));
    renderInventory();
    renderObjects();
  }

  function clearPathResult() {
    state.pathInvestigation = null;
    $('path-result').hidden = true;
    refreshControls();
  }

  function normalizeRef(ref) { return String(ref); }

  function evidenceForRefs(refs) {
    const evidence = (state.snapshot && state.snapshot.evidence) || [];
    const wanted = new Set((refs || []).map(normalizeRef));
    return evidence.filter((item) => wanted.has(normalizeRef(item.id)));
  }

  function tupleLabel(flow) {
    if (!flow) return 'Tuple unavailable';
    const protocol = String(flow.protocol || 'unknown').toUpperCase();
    const ports = (protocol === 'TCP' || protocol === 'UDP') ? `:${flow.source_port || 0} → :${flow.destination_port || 0}` : '';
    return `${flow.source || '?'}${ports.split(' → ')[0] || ''} → ${flow.destination || '?'}${ports.includes(' → ') ? ports.split(' → ')[1] : ''} · ${protocol}`.slice(0, 500);
  }

  function boundedPathText(value, limit = 1200) {
    const text = String(value == null ? '' : value);
    return text.length > limit ? `${text.slice(0, limit)}…` : text;
  }

  function pathDisposition(value) {
    const candidate = String(value || 'indeterminate');
    return ['modeled_forward', 'blocked', 'indeterminate'].includes(candidate) ? candidate : 'indeterminate';
  }

  function appendPathEvidence(container, refs) {
    const uniqueRefs = [...new Set((refs || []).map(normalizeRef))];
    const items = (() => {
      const byID = new Map(((state.snapshot && state.snapshot.evidence) || []).map((item) => [normalizeRef(item.id), item]));
      return uniqueRefs.map((ref) => byID.get(ref)).filter(Boolean);
    })();
    if (!items.length) {
      if ((refs || []).length) container.append(el('span', 'path-no-evidence', 'Source evidence reference unavailable in this snapshot.'));
      return;
    }
    const links = el('div', 'path-evidence-links');
    container.append(links);
    const pageSize = 16;
    let page = 0;
    const pager = el('div', 'pagination-controls path-evidence-pages');
    const previous = el('button', 'text-button', 'Previous sources'); previous.type = 'button';
    const next = el('button', 'text-button', 'Next sources'); next.type = 'button';
    const count = el('span', '');
    const draw = () => {
      links.replaceChildren();
      items.slice(page * pageSize, (page + 1) * pageSize).forEach((item) => links.append(evidenceLink(item)));
      count.textContent = `${page * pageSize + 1}–${Math.min((page + 1) * pageSize, items.length)} of ${items.length} sources`;
      previous.disabled = page === 0; next.disabled = (page + 1) * pageSize >= items.length;
    };
    previous.addEventListener('click', () => { if (page > 0) { page--; draw(); } });
    next.addEventListener('click', () => { if ((page + 1) * pageSize < items.length) { page++; draw(); } });
    if (items.length > pageSize) { pager.append(previous, count, next); container.append(pager); }
    draw();
  }

  function appendPathTuple(container, title, flow) {
    if (!flow || (!flow.source && !flow.destination && !flow.protocol)) return;
    const block = el('div', 'path-tuple');
    block.append(el('span', 'scope-label', title), el('code', '', tupleLabel(flow)));
    container.append(block);
  }

  function renderPathInvestigation(investigation) {
    const result = investigation && investigation.result;
    if (!result || !state.snapshot || investigation.snapshot_id !== state.snapshot.id || result.scope && result.scope.snapshot_id !== state.snapshot.id) {
      showToast('The saved path result does not match the active snapshot. It was not displayed.', 'error');
      return false;
    }
    state.pathInvestigation = investigation;
    $('path-result').hidden = false;
    const disposition = pathDisposition(result.disposition);
    const displayDisposition = disposition.replaceAll('_', ' ').toUpperCase();
    $('path-disposition').textContent = displayDisposition;
    $('path-disposition').className = `path-disposition path-${disposition}`;
    const pathConfig = state.snapshot.path || {};
    $('path-result-scope').textContent = `${state.snapshot.hostname || 'ASA device'} · ${investigation.ingress_interface || result.ingress_interface || 'ingress unavailable'} ingress · ${result.chain_id ? `ACL ${result.chain_id} · ` : ''}${investigation.created_at ? new Date(investigation.created_at).toLocaleString() : 'Saved result'}`;
    const flow = result.original_flow || {};
    $('path-flow-summary').replaceChildren();
    appendPathTuple($('path-flow-summary'), 'ORIGINAL FLOW', flow);
    appendPathTuple($('path-flow-summary'), 'POLICY CHECK', result.policy_flow);
    appendPathTuple($('path-flow-summary'), 'AFTER NAT', result.egress_flow);
    const route = result.selected_route;
    const routeBox = $('path-route-summary'); routeBox.replaceChildren();
    const routeTitle = el('strong', '', result.egress_interface ? `Chosen egress: ${result.egress_interface}` : 'Egress interface not established');
    routeBox.append(routeTitle);
    routeBox.append(el('p', '', boundedPathText(route
      ? `Selected route ${route.prefix || 'prefix unavailable'} via ${route.interface || result.egress_interface || 'interface unavailable'}${route.next_hop ? ` · next hop ${route.next_hop}` : ''}${route.kind ? ` · ${route.kind}` : ''}`
      : 'No single route was selected from the modeled configuration.', 900)));
    routeBox.append(el('p', 'path-model-coverage', `Configuration coverage · routing ${pathConfig.routing_complete ? 'complete' : 'incomplete'} · NAT ${pathConfig.nat_complete ? 'complete' : 'incomplete'}`));
    const ingress = (pathConfig.interfaces || []).find((item) => item.name === (investigation.ingress_interface || result.ingress_interface));
    appendPathEvidence(routeBox, [...((route && route.evidence_refs) || []), ...((ingress && ingress.evidence_refs) || []), ...(result.evidence_refs || []), ...(pathConfig.evidence_refs || [])]);

    const stages = $('path-stages'); stages.replaceChildren();
    const items = Array.isArray(result.stages) ? result.stages.slice(0, 20) : [];
    items.forEach((stage, index) => {
      const card = el('article', `path-stage path-stage-${['matched', 'not_applied', 'blocked', 'unresolved', 'not_evaluated'].includes(stage.status) ? stage.status : 'unresolved'}`);
      const header = el('div', 'path-stage-heading');
      header.append(el('span', 'path-stage-number', String(index + 1).padStart(2, '0')));
      const label = el('div', 'path-stage-title');
      label.append(el('strong', '', stage.name || 'Path stage'), el('span', '', String(stage.status || 'unresolved').replaceAll('_', ' ')));
      header.append(label); card.append(header);
      card.append(el('p', 'path-stage-detail', boundedPathText(stage.detail || 'No stage explanation was returned.')));
      const tuples = el('div', 'path-stage-tuples');
      appendPathTuple(tuples, 'BEFORE', stage.before);
      appendPathTuple(tuples, 'AFTER', stage.after);
      if (tuples.childElementCount) card.append(tuples);
      const candidates = Array.isArray(stage.candidate_ids) ? stage.candidate_ids.slice(0, 12).map((value) => boundedPathText(value, 180)) : [];
      if (candidates.length) card.append(el('p', 'path-candidates', `Candidates: ${candidates.join(', ')}`));
      const stageRefs = stage.name === 'route' && route ? [...(route.evidence_refs || []), ...(stage.evidence_refs || [])] : stage.evidence_refs;
      appendPathEvidence(card, stageRefs);
      stages.append(card);
    });
    if (!items.length) stages.append(el('div', 'path-notice', 'The evaluator returned no path stages.'));
    const renderPathList = (id, values, fallback) => {
      const list = $(id); list.replaceChildren();
      const items = Array.isArray(values) && values.length ? values.slice(0, 20) : [fallback];
      items.forEach((value) => list.append(el('li', '', boundedPathText(value, 1200))));
    };
    const coverageWarnings = (Array.isArray(pathConfig.warnings) ? pathConfig.warnings : []).slice(0, 20)
      .map((warning) => `${warning.code || 'Path coverage'}${warning.line ? ` · physical line ${warning.line}` : ''}: ${warning.message || 'Unsupported path configuration'}`);
    renderPathList('path-reasons', [...(result.reasons || []), ...coverageWarnings], 'No reason details were returned.');
    renderPathList('path-assumptions', result.assumptions, 'No additional assumptions were returned.');
    return true;
  }

  function pathPayload() {
    const protocol = $('path-protocol').value;
    const hasPorts = protocol === 'tcp' || protocol === 'udp';
    const parsePort = (id) => { const value = Number($(id).value); return Number.isInteger(value) ? value : 0; };
    return {
      ingress_interface: $('path-ingress').value,
      flow: {
        source: $('path-source').value.trim(),
        destination: $('path-destination').value.trim(),
        protocol,
        source_port: hasPorts ? parsePort('path-source-port') : 0,
        destination_port: hasPorts ? parsePort('path-destination-port') : 0
      }
    };
  }

  async function evaluatePath() {
    const snapshot = state.snapshot;
    if (!snapshot || !snapshot.path || !$('path-ingress').value) return;
    const requestId = state.scopeRequest;
    const payload = pathPayload();
    const button = $('path-evaluate-button'); setBusy(button, true, 'Tracing…');
    try {
      const body = await request(`/snapshots/${encodeURIComponent(snapshot.id)}/path-evaluate`, { method: 'POST', body: JSON.stringify(payload) });
      if (JSON.stringify(pathPayload()) !== JSON.stringify(payload)) throw new Error('Path inputs changed while the request was running. The response was not displayed.');
      const investigation = body.investigation;
      if (requestId !== state.scopeRequest || state.snapshot !== snapshot || !investigation || investigation.snapshot_id !== snapshot.id || !investigation.result || investigation.result.ingress_interface !== payload.ingress_interface) throw new Error('The path response did not match the selected snapshot and ingress. Refresh before continuing.');
      const flow = investigation.result && investigation.result.original_flow;
      if (!flow || flow.source !== payload.flow.source || flow.destination !== payload.flow.destination || flow.protocol !== payload.flow.protocol || Number(flow.source_port || 0) !== payload.flow.source_port || Number(flow.destination_port || 0) !== payload.flow.destination_port) throw new Error('The path response did not match the submitted tuple. The result was not displayed.');
      renderPathInvestigation(investigation);
      await loadPathHistory();
    } catch (error) { showToast(error.message, 'error'); }
    finally { setBusy(button, false); refreshControls(); }
  }

  function renderPathHistory(items) {
    const list = $('path-history-list'); list.replaceChildren();
    const rows = Array.isArray(items) ? items.slice(0, 8) : [];
    if (!rows.length) { list.append(el('div', 'history-empty', 'No saved path investigations yet.')); return; }
    rows.forEach((item) => {
      const row = el('div', 'history-row');
      const disposition = pathDisposition(item.disposition);
      row.append(el('span', `history-decision path-history-decision path-${disposition}`, String(item.disposition || '?').slice(0, 1).toUpperCase()));
      const details = el('div', 'history-details');
      details.append(el('strong', '', `${item.ingress_interface || 'Ingress unavailable'} · ${item.source || '?'} → ${item.destination || '?'}`));
      details.append(el('span', '', `${item.created_at ? new Date(item.created_at).toLocaleString() : 'Saved'} · ${String(item.disposition || 'indeterminate').replaceAll('_', ' ')} · snapshot ${item.snapshot_id || 'unknown'}`));
      const action = el('button', 'history-open', 'Open path →'); action.type = 'button';
      action.addEventListener('click', () => reopenPathInvestigation(item.id));
      row.append(details, action); list.append(row);
    });
  }

  async function loadPathHistory() {
    const body = await request('/path-investigations');
    renderPathHistory(Array.isArray(body.investigations) ? body.investigations : []);
  }

  async function reopenPathInvestigation(id) {
    if (state.busy) return;
    state.busy = true; refreshControls();
    try {
      const body = await request(`/path-investigations/${encodeURIComponent(id)}`);
      const investigation = body.investigation;
      if (!investigation) throw new Error('Saved path investigation was not returned.');
      await loadSnapshot(investigation.snapshot_id);
      if (!state.snapshot || state.snapshot.id !== investigation.snapshot_id || !state.snapshot.path) throw new Error('Saved path snapshot is unavailable; the result was not opened in a different scope.');
      if (!renderPathInvestigation(investigation)) throw new Error('Saved path result did not match its snapshot.');
      const result = investigation.result || {};
      const flow = result.original_flow || {};
      $('path-ingress').value = investigation.ingress_interface || result.ingress_interface || '';
      $('path-source').value = flow.source || '';
      $('path-destination').value = flow.destination || '';
      $('path-protocol').value = flow.protocol || 'tcp';
      const hasPorts = flow.protocol === 'tcp' || flow.protocol === 'udp';
      $('path-source-port').value = hasPorts ? String(flow.source_port || '') : '';
      $('path-destination-port').value = hasPorts ? String(flow.destination_port || '') : '';
      updatePathInterfaceMeta();
      $('path-result').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    } catch (error) { showToast(error.message, 'error'); }
    finally { state.busy = false; refreshControls(); }
  }

  function networkFormValid() {
    const nodes = [...document.querySelectorAll('.network-node-row')];
    if (nodes.length < 2 || nodes.length > 16) return false;
    const ids = nodes.map((row) => row.querySelector('.network-node-id').value.trim());
    if (ids.some((id) => !id) || new Set(ids).size !== ids.length || nodes.some((row) => !row.querySelector('.network-snapshot-id').value)) return false;
    const source = $('network-source-node').value; const destination = $('network-destination-node').value;
    if (!ids.includes(source) || !ids.includes(destination) || !source || !destination) return false;
    if (!$('network-source-interface').value.trim() || !$('network-destination-interface').value.trim() || !$('network-flow-source').value.trim() || !$('network-flow-destination').value.trim()) return false;
    return [...document.querySelectorAll('.network-link-row')].length <= 32 && [...document.querySelectorAll('.network-link-row')].every((row) => row.querySelector('.network-link-a-node').value && row.querySelector('.network-link-b-node').value && row.querySelector('.network-link-a-interface').value.trim() && row.querySelector('.network-link-b-interface').value.trim());
  }

  function clearNetworkResult() { state.networkInvestigation = null; $('network-result').hidden = true; clearExplorationResult(); refreshControls(); }

  function clearExplorationResult() { state.networkExploration = null; if ($('exploration-result')) $('exploration-result').hidden = true; }

  function networkNodeRows() { return [...document.querySelectorAll('.network-node-row')]; }
  function nodeIDs() { return networkNodeRows().map((row) => row.querySelector('.network-node-id').value.trim()).filter(Boolean); }

  function refreshNetworkTopology() {
    const ids = nodeIDs();
    document.querySelectorAll('.network-link-a-node,.network-link-b-node,#network-source-node,#network-destination-node').forEach((select) => {
      const current = select.value;
      select.replaceChildren(new Option('Select device', ''), ...ids.map((id) => new Option(id, id)));
      if (ids.includes(current)) select.value = current;
    });
    networkNodeRows().forEach((row) => {
      const select = row.querySelector('.network-snapshot-id'); const current = select.value;
      select.replaceChildren(new Option('Select snapshot', ''), ...state.snapshots.map((snapshot) => new Option(summaryName(snapshot), snapshot.id)));
      if (state.snapshots.some((snapshot) => snapshot.id === current)) select.value = current;
    });
    refreshControls();
  }

  function addNetworkNode(node = {}) {
    if (networkNodeRows().length >= 16) return;
    const row = el('div', 'network-row network-node-row');
    const idWrap = el('label', 'field'); idWrap.append(el('span', '', 'DEVICE ID'));
    const id = document.createElement('input'); id.className = 'network-node-id'; id.value = node.node_id || `firewall-${networkNodeRows().length + 1}`; id.autocomplete = 'off'; idWrap.append(id);
    const snapWrap = el('label', 'field'); snapWrap.append(el('span', '', 'CONFIGURATION SNAPSHOT'));
    const snapshot = document.createElement('select'); snapshot.className = 'network-snapshot-id'; snapWrap.append(snapshot);
    const remove = el('button', 'small-icon-button network-remove-row', '×'); remove.type = 'button'; remove.setAttribute('aria-label', 'Remove device');
    row.append(idWrap, snapWrap, remove); $('network-nodes').append(row);
    const update = () => { refreshNetworkTopology(); clearNetworkResult(); };
    id.addEventListener('input', update); snapshot.addEventListener('change', update);
    remove.addEventListener('click', () => { row.remove(); refreshNetworkTopology(); clearNetworkResult(); });
    refreshNetworkTopology();
    if (node.snapshot_id) snapshot.value = node.snapshot_id;
  }

  function addNetworkLink(link = {}) {
    if (document.querySelectorAll('.network-link-row').length >= 32) return;
    const row = el('div', 'network-row network-link-row');
    const field = (label, cls, value, select = false) => {
      const wrap = el('label', 'field'); wrap.append(el('span', '', label));
      const input = select ? document.createElement('select') : document.createElement('input'); input.className = cls; if (select) input.add(new Option('Select device', '')); input.value = value || ''; input.autocomplete = 'off'; wrap.append(input); return wrap;
    };
    const aNode = field('END A DEVICE', 'network-link-a-node', link.a && link.a.node_id, true);
    const aInterface = field('END A INTERFACE', 'network-link-a-interface', link.a && link.a.interface);
    const bNode = field('END B DEVICE', 'network-link-b-node', link.b && link.b.node_id, true);
    const bInterface = field('END B INTERFACE', 'network-link-b-interface', link.b && link.b.interface);
    const remove = el('button', 'small-icon-button network-remove-row', '×'); remove.type = 'button'; remove.setAttribute('aria-label', 'Remove link');
    row.append(aNode, aInterface, el('span', 'network-link-arrow', '↔'), bNode, bInterface, remove); $('network-links').append(row);
    row.querySelectorAll('input,select').forEach((input) => input.addEventListener('input', () => { clearNetworkResult(); refreshControls(); }));
    row.querySelectorAll('select').forEach((input) => input.addEventListener('change', () => { clearNetworkResult(); refreshControls(); }));
    remove.addEventListener('click', () => { row.remove(); clearNetworkResult(); refreshControls(); });
    refreshNetworkTopology();
    if (link.a) row.querySelector('.network-link-a-node').value = link.a.node_id;
    if (link.b) row.querySelector('.network-link-b-node').value = link.b.node_id;
    refreshControls();
  }

  function networkPayload() {
    const protocol = $('network-flow-protocol').value; const hasPorts = protocol === 'tcp' || protocol === 'udp';
    return {
      nodes: networkNodeRows().map((row) => ({ node_id: row.querySelector('.network-node-id').value.trim(), snapshot_id: row.querySelector('.network-snapshot-id').value })),
      links: [...document.querySelectorAll('.network-link-row')].map((row) => ({ a: { node_id: row.querySelector('.network-link-a-node').value, interface: row.querySelector('.network-link-a-interface').value.trim() }, b: { node_id: row.querySelector('.network-link-b-node').value, interface: row.querySelector('.network-link-b-interface').value.trim() } })),
      source: { node_id: $('network-source-node').value, interface: $('network-source-interface').value.trim() },
      destination: { node_id: $('network-destination-node').value, interface: $('network-destination-interface').value.trim() },
      flow: { source: $('network-flow-source').value.trim(), destination: $('network-flow-destination').value.trim(), protocol, source_port: hasPorts ? Number($('network-flow-source-port').value) || 0 : 0, destination_port: hasPorts ? Number($('network-flow-destination-port').value) || 0 : 0 }
    };
  }

  function networkRequestJSON(value) {
    const normalized = JSON.parse(JSON.stringify(value));
    if (normalized && normalized.flow) {
      if (Number(normalized.flow.source_port) === 0) delete normalized.flow.source_port;
      if (Number(normalized.flow.destination_port) === 0) delete normalized.flow.destination_port;
    }
    const sort = (item) => Array.isArray(item) ? item.map(sort) : item && typeof item === 'object'
      ? Object.fromEntries(Object.keys(item).sort().map((key) => [key, sort(item[key])])) : item;
    return JSON.stringify(sort(normalized));
  }

  function appendNetworkEvidence(container, refs, expectedRecord) {
    const seen = new Set();
    const items = (Array.isArray(refs) ? refs : []).filter((ref) => { const key = `${ref.snapshot_id}:${ref.ref}`; if (seen.has(key)) return false; seen.add(key); return true; });
    if (!items.length) { container.append(el('span', 'path-no-evidence', 'No source references were returned for this hop.')); return; }
    const links = el('div', 'path-evidence-links'); container.append(links);
    let page = 0; const size = 16; const pager = el('div', 'pagination-controls path-evidence-pages');
    const prev = el('button', 'text-button', 'Previous sources'); prev.type = 'button'; const next = el('button', 'text-button', 'Next sources'); next.type = 'button'; const count = el('span');
    const draw = () => { links.replaceChildren(); items.slice(page * size, (page + 1) * size).forEach((ref) => {
      const cached = state.networkSnapshots.get(ref.snapshot_id); const item = cached && (cached.evidence || []).find((entry) => entry.id === ref.ref);
      const button = el('button', 'evidence-link', item ? `${cached.hostname || ref.snapshot_id.slice(0, 8)} · ${item.kind || 'Source'} · lines ${item.start_line}–${item.end_line}` : `Source ${String(ref.snapshot_id).slice(0, 8)} · ${ref.ref}`); button.type = 'button'; button.title = item && item.summary || String(ref.ref || ''); button.dataset.snapshotId = ref.snapshot_id; button.dataset.evidenceRef = ref.ref; button.addEventListener('click', async () => {
        try { const snap = await networkSnapshot(ref.snapshot_id); if (!expectedRecord || (state.networkInvestigation !== expectedRecord && state.networkExploration !== expectedRecord)) throw new Error('The network result changed before its source was opened.'); const evidence = (snap.evidence || []).find((entry) => String(entry.id) === String(ref.ref)); if (!evidence) throw new Error('Evidence reference is unavailable in its snapshot.'); showEvidence(evidence, button, snap); }
        catch (error) { showToast(error.message, 'error'); }
      }); links.append(button); });
      count.textContent = `${page * size + 1}–${Math.min((page + 1) * size, items.length)} of ${items.length} sources`; prev.disabled = page === 0; next.disabled = (page + 1) * size >= items.length; };
    prev.addEventListener('click', () => { if (page > 0) { page--; draw(); } }); next.addEventListener('click', () => { if ((page + 1) * size < items.length) { page++; draw(); } });
    if (items.length > size) { pager.append(prev, count, next); container.append(pager); } draw();
  }

  async function networkSnapshot(id) {
    if (state.networkSnapshots.has(id)) return state.networkSnapshots.get(id);
    const body = await request(`/snapshots/${encodeURIComponent(id)}`); if (!body.snapshot) throw new Error('A referenced device snapshot is unavailable.');
    state.networkSnapshots.set(id, body.snapshot); return body.snapshot;
  }

  function renderNetworkHops(container, title, hops, expectedRecord = state.networkInvestigation) {
    container.replaceChildren(el('h3', '', title));
    (hops || []).forEach((hop, index) => {
      const card = el('article', 'path-stage'); card.tabIndex = -1; card.dataset.nodeId = hop.node_id || ''; const heading = el('div', 'path-stage-heading');
      heading.append(el('span', 'path-stage-number', String(index + 1).padStart(2, '0')));
      const owner = state.networkSnapshots.get(hop.snapshot_id);
      const label = el('div', 'path-stage-title'); label.append(el('strong', '', `${hop.node_id || 'Firewall'} · ${vendorName(owner && owner.vendor)}`), el('span', '', hop.path && hop.path.disposition || 'unknown')); heading.append(label); card.append(heading);
      const path = hop.path || {}; const route = path.selected_route;
      card.append(el('p', 'path-stage-detail', `Snapshot ${hop.snapshot_id || 'unknown'} · ingress ${path.ingress_interface || path.ingress || 'unavailable'} · egress ${path.egress_interface || 'unavailable'}${route ? ` · route ${route.prefix || 'unknown'} via ${route.interface || 'unknown'}${route.next_hop ? ` · next hop ${route.next_hop}` : ''}${route.kind ? ` · ${route.kind}` : ''}` : ''}`));
      const snapshot = state.networkSnapshots.get(hop.snapshot_id); const warnings = snapshot && snapshot.path && snapshot.path.warnings || [];
      warnings.slice(0, 20).forEach((warning) => card.append(el('p', 'path-stage-detail', `${warning.code || 'Warning'} · physical line ${warning.line || '?'}: ${warning.message || 'Unsupported path configuration'}`)));
      if (warnings.length > 20) card.append(el('p', 'path-stage-detail', `${warnings.length - 20} additional configuration warnings; inspect the snapshot source and exported record.`));
      const tuples = el('div', 'path-stage-tuples'); appendPathTuple(tuples, 'ORIGINAL', path.original_flow); appendPathTuple(tuples, 'POLICY', path.policy_flow); appendPathTuple(tuples, 'EGRESS', path.egress_flow || path.final_flow); if (tuples.childElementCount) card.append(tuples);
      const qualify = (refs) => (refs || []).map((ref) => typeof ref === 'object' && ref !== null ? ref : ({ snapshot_id: hop.snapshot_id, ref }));
      const stages = Array.isArray(path.stages) ? path.stages : Array.isArray(path.hops) ? path.hops : [];
      stages.forEach((stage) => {
        card.append(el('p', 'path-stage-detail', `${stage.name || stage.kind || 'Stage'} · ${stage.status || stage.disposition || 'unavailable'}: ${boundedPathText(stage.detail || stage.reason || '', 700)}`));
        const stageTuples = el('div', 'path-stage-tuples'); appendPathTuple(stageTuples, 'BEFORE', stage.before); appendPathTuple(stageTuples, 'AFTER', stage.after); if (stageTuples.childElementCount) card.append(stageTuples);
        const stageRefs = [...qualify(stage.evidence_refs), ...(stage.evidence || [])];
        if (stageRefs.length) { const evidence = el('div', 'path-stage-detail'); evidence.append(el('strong', '', 'Stage evidence')); appendNetworkEvidence(evidence, stageRefs, expectedRecord); card.append(evidence); }
      });
      const refs = [...(hop.evidence || []), ...qualify(path.evidence_refs), ...qualify(route && route.evidence_refs)]; if (refs.length) { const evidence = el('div', 'path-stage-detail'); evidence.append(el('strong', '', 'Hop evidence')); appendNetworkEvidence(evidence, refs, expectedRecord); card.append(evidence); } container.append(card);
    });
    if (!(hops || []).length) container.append(el('p', 'path-stage-detail', 'No hops were returned.'));
  }

  function renderNetworkInvestigation(investigation) {
    if (!investigation || !investigation.result) throw new Error('Network investigation response was incomplete.');
    state.networkInvestigation = investigation; const result = investigation.result;
    $('network-result').hidden = false; $('network-disposition').textContent = String(result.disposition || 'indeterminate').replaceAll('_', ' ').toUpperCase();
    $('network-disposition').className = `path-disposition path-${String(result.disposition || 'indeterminate').replaceAll('_', '-')}`;
    $('network-result-scope').textContent = `${investigation.created_at ? new Date(investigation.created_at).toLocaleString() : 'Saved result'} · ${investigation.id || ''}`;
    $('network-flow-summary').replaceChildren(); appendPathTuple($('network-flow-summary'), 'ORIGINAL FLOW', result.original_flow || investigation.request && investigation.request.flow); appendPathTuple($('network-flow-summary'), 'FORWARD FINAL FLOW', result.forward && result.forward.final_flow); appendPathTuple($('network-flow-summary'), 'RETURN FINAL FLOW', result.return && result.return.final_flow);
    renderNetworkHops($('network-forward-hops'), 'Forward path', result.forward && result.forward.hops, investigation);
    renderNetworkHops($('network-return-hops'), 'Return path', result.return && result.return.hops, investigation);
    const fill = (id, values, fallback) => { const list = $(id); list.replaceChildren(); (Array.isArray(values) && values.length ? values : [fallback]).forEach((value) => list.append(el('li', '', boundedPathText(value, 1200)))); };
    fill('network-reasons', [...(result.reasons || []), ...((result.forward && result.forward.reasons) || []), ...((result.return && result.return.reasons) || [])], 'No reason details were returned.');
    fill('network-assumptions', result.assumptions, 'No additional assumptions were returned.'); refreshControls();
  }

  function renderExplorationTopology(exploration, branch, returnCandidate) {
    const svg = $('exploration-topology'); svg.replaceChildren();
    const request = exploration.request || {}; const nodes = request.nodes || []; const links = request.links || [];
    const forward = branch && branch.forward && branch.forward.hops || []; const returns = returnCandidate && returnCandidate.direction && returnCandidate.direction.hops || [];
    const edgeKey = (link) => { if (!link || !link.a || !link.b) return ''; const endpoint = (value) => `${value.node_id}\u0000${String(value.interface || '').toLowerCase()}`; return [endpoint(link.a), endpoint(link.b)].sort().join('\u0001'); };
    const edgeDirection = (link, fromNode) => link && link.a && link.a.node_id === fromNode ? [link.a, link.b] : [link && link.b, link && link.a];
    const adjacency = new Map(nodes.map((node) => [node.node_id, []]));
    links.forEach((link) => { if (!link.a || !link.b) return; if (adjacency.has(link.a.node_id) && adjacency.has(link.b.node_id)) { adjacency.get(link.a.node_id).push(link.b.node_id); adjacency.get(link.b.node_id).push(link.a.node_id); } });
    const source = request.source && request.source.node_id; const destination = request.destination && request.destination.node_id; const layerByNode = new Map();
    if (adjacency.has(source)) { layerByNode.set(source, 0); const queue = [source]; while (queue.length) { const current = queue.shift(); adjacency.get(current).forEach((peer) => { if (!layerByNode.has(peer)) { layerByNode.set(peer, layerByNode.get(current) + 1); queue.push(peer); } }); } }
    let maxLayer = Math.max(0, ...layerByNode.values()); nodes.forEach((node) => { if (!layerByNode.has(node.node_id)) layerByNode.set(node.node_id, maxLayer + 1); });
    if (destination && layerByNode.has(destination)) { const dLayer = layerByNode.get(destination); if (dLayer < maxLayer) { nodes.filter((node) => layerByNode.get(node.node_id) > dLayer).forEach((node) => layerByNode.set(node.node_id, layerByNode.get(node.node_id) + 1)); layerByNode.set(destination, maxLayer); } }
    maxLayer = Math.max(1, ...layerByNode.values()); const byLayer = new Map(); nodes.forEach((node) => { const layer = layerByNode.get(node.node_id) || 0; if (!byLayer.has(layer)) byLayer.set(layer, []); byLayer.get(layer).push(node); });
    const maxLayerNodes = Math.max(1, ...[...byLayer.values()].map((members) => members.length)); const svgHeight = Math.max(250, maxLayerNodes * 68 + 30); svg.setAttribute('viewBox', `0 0 720 ${svgHeight}`);
    const positions = new Map(); byLayer.forEach((members, layer) => members.forEach((node, index) => { const y = members.length === 1 ? svgHeight / 2 : 30 + index * ((svgHeight - 60) / (members.length - 1)); positions.set(node.node_id, { x: node.node_id === destination ? 665 : 55 + layer * (610 / maxLayer), y }); }));
    const ns = 'http://www.w3.org/2000/svg'; const makeLine = (x1, y1, x2, y2, cls, title, arrow = '') => { const item = document.createElementNS(ns, 'line'); item.setAttribute('x1', x1); item.setAttribute('y1', y1); item.setAttribute('x2', x2); item.setAttribute('y2', y2); item.setAttribute('class', cls); if (arrow) item.setAttribute('marker-end', `url(#explore-arrow-${arrow})`); const tip = document.createElementNS(ns, 'title'); tip.textContent = title; item.append(tip); svg.append(item); };
    const defs = document.createElementNS(ns, 'defs'); [['forward','#43865c'],['return','#567da5'],['blocked','#b54e55'],['unknown','#c08b32']].forEach(([id,color]) => { const marker = document.createElementNS(ns,'marker'); marker.setAttribute('id',`explore-arrow-${id}`); marker.setAttribute('viewBox','0 0 10 10'); marker.setAttribute('refX','8'); marker.setAttribute('refY','5'); marker.setAttribute('markerWidth','5'); marker.setAttribute('markerHeight','5'); marker.setAttribute('orient','auto-start-reverse'); const path = document.createElementNS(ns,'path'); path.setAttribute('d','M 0 0 L 10 5 L 0 10 z'); path.setAttribute('fill',color); marker.append(path); defs.append(marker); }); svg.append(defs);
    const edgeClass = (status, direction) => { const value = String(status || 'unknown').toLowerCase(); return `topology-edge topology-${direction} topology-state-${value.replace(/[^a-z0-9-]/g, '-')}`; };
    const drawPathEdges = (hops, direction, status) => (hops || []).forEach((hop) => { if (!hop.link) return; const key = edgeKey(hop.link); const selected = links.find((link) => edgeKey(link) === key); if (!selected) return; const [from, to] = edgeDirection(selected, hop.node_id); const a = from && positions.get(from.node_id); const b = to && positions.get(to.node_id); if (!a || !b) return; const dx = b.x - a.x; const dy = b.y - a.y; const length = Math.max(1, Math.hypot(dx, dy)); const offset = direction === 'forward' ? -5 : 5; const ox = -dy / length * offset; const oy = dx / length * offset; const value = String(status || 'unknown').toLowerCase(); const arrow = value === 'blocked' ? 'blocked' : ['unknown','indeterminate','unresolved','not_evaluated'].includes(value) ? 'unknown' : direction; makeLine(a.x + ox, a.y + oy, b.x + ox, b.y + oy, edgeClass(status, direction), `Selected ${direction} path · ${from.node_id} (${from.interface}) → ${to.node_id} (${to.interface}) · ${status || 'unknown'}`, arrow); });
    links.forEach((link) => { const a = positions.get(link.a.node_id); const b = positions.get(link.b.node_id); if (!a || !b) return; makeLine(a.x, a.y, b.x, b.y, 'topology-edge', `Operator supplied link ${link.a.node_id} ${link.a.interface} ↔ ${link.b.node_id} ${link.b.interface}`); });
    drawPathEdges(forward, 'forward', branch && branch.forward && branch.forward.disposition); drawPathEdges(returns, 'return', returnCandidate && returnCandidate.disposition);
    const hitsByNode = new Map(); const addHits = (hops, direction) => (hops || []).forEach((hop, index) => { if (!hitsByNode.has(hop.node_id)) hitsByNode.set(hop.node_id, []); hitsByNode.get(hop.node_id).push({ hop, direction, index }); }); addHits(forward, 'forward'); addHits(returns, 'return');
    const statusClass = (value) => String(value || 'unknown').toLowerCase().replace(/[^a-z0-9-]/g, '-');
    const displayNode = (value) => value.length > 16 ? `${value.slice(0, 13)}…` : value;
    nodes.forEach((node) => { const pos = positions.get(node.node_id); const hits = hitsByNode.get(node.node_id) || []; const hit = hits[0]; const hopStatus = hit && hit.hop.path && hit.hop.path.disposition || 'unknown'; const directions = [...new Set(hits.map((item) => item.direction))]; const group = document.createElementNS(ns, 'g'); group.setAttribute('class', `topology-node ${directions.map((direction) => `topology-${direction}`).join(' ')} topology-state-${statusClass(hopStatus)}`); group.setAttribute('tabindex', '0'); group.setAttribute('role', 'button'); group.setAttribute('aria-label', `${node.node_id}, operator supplied topology node${hits.length ? `, selected ${directions.join(' and ')} hop${hits.length === 1 ? ` ${hit.index + 1}` : 's'}${hopStatus ? `, status ${hopStatus}` : ''}` : ', not in selected branch'}`); group.dataset.nodeId = node.node_id; const circle = document.createElementNS(ns, 'circle'); circle.setAttribute('cx', pos.x); circle.setAttribute('cy', pos.y); circle.setAttribute('r', '25'); const label = document.createElementNS(ns, 'text'); label.setAttribute('x', pos.x); label.setAttribute('y', pos.y + 43); label.textContent = displayNode(node.node_id); const title = document.createElementNS(ns, 'title'); title.textContent = node.node_id; label.append(title); const status = document.createElementNS(ns, 'text'); status.setAttribute('x', pos.x); status.setAttribute('y', pos.y + 59); status.setAttribute('class', 'topology-node-status'); status.textContent = hits.length ? `${directions.join('+')} · ${hopStatus}` : 'declared'; group.append(circle, label, status); const focus = () => focusExplorationHop(node.node_id); group.addEventListener('click', focus); group.addEventListener('keydown', (event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); focus(); } }); svg.append(group); });
    const list = $('exploration-topology-list'); list.replaceChildren(); nodes.forEach((node) => { const button = el('button', 'topology-list-node', node.node_id); button.type = 'button'; button.addEventListener('click', () => focusExplorationHop(node.node_id)); const hits = hitsByNode.get(node.node_id) || []; button.setAttribute('aria-label', `${node.node_id}${hits.length ? `, ${[...new Set(hits.map((item) => item.direction))].join(' and ')} hop${hits.length === 1 ? ` ${hits[0].index + 1}` : 's'}` : ', declared node without selected hop'}`); list.append(button); });
    links.forEach((link) => list.append(el('span', 'topology-list-link', `${link.a.node_id} (${link.a.interface}) ↔ ${link.b.node_id} (${link.b.interface})`)));
  }

  function focusExplorationHop(nodeId) {
    const forward = [...document.querySelectorAll('#exploration-forward-hops .path-stage')].find((item) => item.dataset.nodeId === nodeId);
    const returned = [...document.querySelectorAll('#exploration-return-hops .path-stage')].find((item) => item.dataset.nodeId === nodeId);
    const card = forward || returned;
    if (card) { card.scrollIntoView({ behavior: 'smooth', block: 'center' }); card.focus({ preventScroll: true }); }
    else showToast(`${nodeId} is declared in the topology but has no hop in the selected branch.`);
  }

  function renderNetworkExploration(exploration) {
    if (!exploration || !exploration.result) throw new Error('Network exploration response was incomplete.');
    state.networkExploration = exploration; $('exploration-result').hidden = false;
    const result = exploration.result; const branches = result.branches || []; const branchSelect = $('exploration-branch'); const returnSelect = $('exploration-return');
    const pathLabel = (hops) => { const ids = (hops || []).map((hop) => hop.node_id).filter(Boolean); const compact = ids.length > 4 ? [...ids.slice(0, 2), '…', ids[ids.length - 1]] : ids; return compact.join(' → ') || 'path unavailable'; };
    $('exploration-disposition').textContent = String(result.disposition || 'indeterminate').replaceAll('_', ' ').toUpperCase(); $('exploration-disposition').className = `path-disposition path-${String(result.disposition || 'indeterminate').replaceAll('_', '-')}`;
    $('exploration-summary').textContent = `${exploration.created_at ? new Date(exploration.created_at).toLocaleString() : 'Saved result'} · ${exploration.id || ''} · ${branches.length} branches`;
    $('exploration-coverage').textContent = `Candidate search coverage: ${result.complete ? 'complete; candidate enumeration finished' : 'incomplete'} · ${result.truncated ? 'some candidates were omitted by limits' : 'not truncated'}${(result.reasons || []).length ? ` · ${(result.reasons || []).join(' ')}` : ''}`;
    branchSelect.replaceChildren(...branches.map((branch, index) => new Option(`Branch ${index + 1} · ${pathLabel(branch.forward && branch.forward.hops)} · ${branch.disposition || 'indeterminate'}`, String(index))));
    const branch = () => branches[Number(branchSelect.value)] || null;
    const candidatesForBranch = () => branch() && branch().returns || [];
    const draw = () => { const selectedBranch = branch(); const candidates = candidatesForBranch(); const candidate = candidates[Number(returnSelect.value)] || null;
      $('exploration-return-status').textContent = candidate ? `Return candidate ${candidate.disposition || 'indeterminate'} · session ${candidate.session_status || 'unknown'}${candidate.asymmetric ? ' · asymmetric route' : ''}. A route candidate does not prove an established session or permit.` : 'No return candidate is selected. Route analysis does not prove an established session or permit.';
      $('exploration-return-status').className = `exploration-return-status exploration-return-${String(candidate && candidate.disposition || 'unknown').replace(/[^a-z0-9-]/gi, '-')} exploration-session-${String(candidate && candidate.session_status || 'unknown').replace(/[^a-z0-9-]/gi, '-')}${candidate && candidate.asymmetric ? ' exploration-return-asymmetric' : ''}`;
      $('exploration-flow-summary').replaceChildren(); appendPathTuple($('exploration-flow-summary'), 'ORIGINAL FLOW', result.original_flow || exploration.request && exploration.request.flow); appendPathTuple($('exploration-flow-summary'), 'FORWARD FINAL FLOW', selectedBranch && selectedBranch.forward && selectedBranch.forward.final_flow); appendPathTuple($('exploration-flow-summary'), 'RETURN FINAL FLOW', candidate && candidate.direction && candidate.direction.final_flow);
      renderNetworkHops($('exploration-forward-hops'), selectedBranch ? `Forward branch ${Number(branchSelect.value) + 1} · ${selectedBranch.forward && selectedBranch.forward.disposition || 'indeterminate'}` : 'Forward branch', selectedBranch && selectedBranch.forward && selectedBranch.forward.hops, exploration);
      renderNetworkHops($('exploration-return-hops'), candidate ? `Return candidate ${Number(returnSelect.value) + 1} · ${candidate.disposition || 'indeterminate'}` : 'Return candidate', candidate && candidate.direction && candidate.direction.hops, exploration);
      const fill = (id, values, fallback) => { const list = $(id); list.replaceChildren(); (values && values.length ? values : [fallback]).forEach((value) => list.append(el('li', '', boundedPathText(value, 1200)))); };
      fill('exploration-reasons', [...(result.reasons || []), ...(selectedBranch && selectedBranch.reasons || []), ...(selectedBranch && selectedBranch.forward && selectedBranch.forward.reasons || []), ...(candidate && candidate.reasons || []), ...(candidate && candidate.direction && candidate.direction.reasons || [])], 'No reason details were returned.'); fill('exploration-assumptions', [...(result.assumptions || []), ...(result.truncated ? ['Some branches were omitted because the exploration was truncated.'] : [])], 'No additional assumptions were returned.');
      renderExplorationTopology(exploration, selectedBranch, candidate);
    };
    const rebuildReturns = () => { const candidates = candidatesForBranch(); returnSelect.replaceChildren(...(candidates.length ? candidates.map((candidate, index) => new Option(`Return ${index + 1} · ${pathLabel(candidate.direction && candidate.direction.hops)} · session ${candidate.session_status || 'unknown'}${candidate.asymmetric ? ' · asymmetric' : ''}`, String(index))) : [new Option('No return candidate', '')])); if (candidates.length) returnSelect.value = '0'; draw(); };
    branchSelect.onchange = rebuildReturns; returnSelect.onchange = draw; rebuildReturns(); refreshControls();
  }

  async function exploreNetwork() {
    if (!networkFormValid()) return; const payload = networkPayload(); const submitted = JSON.stringify(payload); const button = $('network-explore'); setBusy(button, true, 'Exploring…');
    try { const body = await request('/network-explorations', { method: 'POST', body: submitted }); await Promise.all(payload.nodes.map((node) => networkSnapshot(node.snapshot_id))); if (JSON.stringify(networkPayload()) !== submitted) throw new Error('Network topology or flow changed while the request was running. The response was not displayed.'); if (!body.exploration || !body.exploration.result || networkRequestJSON(body.exploration.request) !== networkRequestJSON(payload)) throw new Error('The exploration response did not match the submitted topology. The result was not displayed.'); renderNetworkExploration(body.exploration); await loadExplorationHistory(); }
    catch (error) { showToast(error.message, 'error'); } finally { setBusy(button, false); refreshControls(); }
  }

  async function evaluateNetwork() {
    if (!networkFormValid()) return;
    const payload = networkPayload(); const submitted = JSON.stringify(payload); const button = $('network-evaluate'); setBusy(button, true, 'Tracing…');
    try {
      const body = await request('/network-investigations', { method: 'POST', body: submitted });
      await Promise.all(payload.nodes.map((node) => networkSnapshot(node.snapshot_id)));
      if (JSON.stringify(networkPayload()) !== submitted) throw new Error('Network topology or flow changed while the request was running. The response was not displayed.');
      if (!body.investigation || !body.investigation.result || networkRequestJSON(body.investigation.request) !== networkRequestJSON(payload)) throw new Error('The network response did not match the submitted topology. The result was not displayed.');
      renderNetworkInvestigation(body.investigation); await loadNetworkHistory();
    } catch (error) { showToast(error.message, 'error'); }
    finally { setBusy(button, false); refreshControls(); }
  }

  function renderNetworkHistory(items) {
    const list = $('network-history-list'); list.replaceChildren();
    if (!items.length) { list.append(el('div', 'history-empty', 'No saved multi-firewall investigations yet.')); return; }
    items.slice(0, 8).forEach((item) => { const row = el('div', 'history-row'); const disposition = String(item.disposition || 'indeterminate').replaceAll('_', '-');
      row.append(el('span', `history-decision path-history-decision path-${disposition}`, String(item.disposition || '?').slice(0, 1).toUpperCase()));
      const details = el('div', 'history-details'); details.append(el('strong', '', `${item.source && item.source.node_id || '?'} → ${item.destination && item.destination.node_id || '?'} · ${item.node_count || 0} devices`)); details.append(el('span', '', `${item.created_at ? new Date(item.created_at).toLocaleString() : 'Saved'} · ${item.disposition || 'indeterminate'}`));
      const open = el('button', 'history-open network-history-open', 'Open →'); open.type = 'button'; open.addEventListener('click', () => reopenNetworkInvestigation(item.id)); row.append(details, open); list.append(row);
    });
  }

  async function loadNetworkHistory() { const body = await request('/network-investigations'); renderNetworkHistory(Array.isArray(body.investigations) ? body.investigations : []); }

  async function reopenNetworkInvestigation(id) {
    if (state.busy) return; const before = JSON.stringify(networkPayload()); state.busy = true; refreshControls();
    try { const body = await request(`/network-investigations/${encodeURIComponent(id)}`); const investigation = body.investigation; if (!investigation || !investigation.request) throw new Error('Saved network investigation was incomplete.');
      await Promise.all(investigation.request.nodes.map((node) => networkSnapshot(node.snapshot_id)));
      if (JSON.stringify(networkPayload()) !== before) throw new Error('Network topology or flow changed while history was loading. The saved result was not displayed.');
      const req = investigation.request; $('network-nodes').replaceChildren(); $('network-links').replaceChildren(); req.nodes.forEach(addNetworkNode); req.links.forEach(addNetworkLink); refreshNetworkTopology();
      req.nodes.forEach((node, index) => { const row = networkNodeRows()[index]; row.querySelector('.network-node-id').value = node.node_id; row.querySelector('.network-snapshot-id').value = node.snapshot_id; });
      req.links.forEach((link, index) => { const row = document.querySelectorAll('.network-link-row')[index]; row.querySelector('.network-link-a-node').value = link.a.node_id; row.querySelector('.network-link-a-interface').value = link.a.interface; row.querySelector('.network-link-b-node').value = link.b.node_id; row.querySelector('.network-link-b-interface').value = link.b.interface; });
      refreshNetworkTopology(); $('network-source-node').value = req.source.node_id; $('network-source-interface').value = req.source.interface; $('network-destination-node').value = req.destination.node_id; $('network-destination-interface').value = req.destination.interface;
      $('network-flow-source').value = req.flow.source; $('network-flow-destination').value = req.flow.destination; $('network-flow-protocol').value = req.flow.protocol; $('network-flow-source-port').value = req.flow.source_port || ''; $('network-flow-destination-port').value = req.flow.destination_port || '';
      renderNetworkInvestigation(investigation); $('network-result').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    } catch (error) { showToast(error.message, 'error'); } finally { state.busy = false; refreshControls(); }
  }

  async function loadNetworkSample(endpoint = '/sample/network') {
    const button = $(endpoint === '/sample/network' ? 'network-load-sample' : endpoint === '/sample/network-exploration' ? 'network-load-exploration-sample' : 'network-load-vendor-sample'); setBusy(button, true, 'Importing sample…');
    try {
      const sample = await request(endpoint); if (!Array.isArray(sample.configs) || sample.configs.length < 2) throw new Error('The multi-firewall sample was incomplete.');
      const imported = [];
      for (const device of sample.configs) {
        const response = await request('/imports', { method: 'POST', body: JSON.stringify({ name: device.name || `${device.node_id}.cfg`, config: device.config, vendor: device.vendor || 'auto' }) });
        if (!response.snapshot || !response.snapshot.id) throw new Error(`The ${device.node_id || 'sample'} configuration did not return a snapshot.`);
        state.networkSnapshots.set(response.snapshot.id, response.snapshot);
        imported.push({ node_id: device.node_id, snapshot_id: response.snapshot.id });
      }
      const snapshots = await request('/snapshots'); state.snapshots = Array.isArray(snapshots.snapshots) ? snapshots.snapshots : [];
      await loadSnapshots(state.snapshot && state.snapshot.id || '');
      $('network-nodes').replaceChildren(); $('network-links').replaceChildren(); imported.forEach(addNetworkNode);
      imported.forEach((node, index) => { const row = networkNodeRows()[index]; row.querySelector('.network-node-id').value = node.node_id; row.querySelector('.network-snapshot-id').value = node.snapshot_id; });
      (sample.links || []).forEach(addNetworkLink); refreshNetworkTopology();
      $('network-source-node').value = sample.source.node_id; $('network-source-interface').value = sample.source.interface; $('network-destination-node').value = sample.destination.node_id; $('network-destination-interface').value = sample.destination.interface;
      Object.entries(sample.flow || {}).forEach(([key, value]) => { const id = `network-flow-${key.replaceAll('_', '-')}`; if ($(id)) $(id).value = String(value); });
      $('network-status').textContent = `${imported.length} sample firewall snapshots imported. Review device IDs and operator supplied links before tracing.`;
      clearNetworkResult(); refreshControls();
    } catch (error) { showToast(error.message, 'error'); $('network-status').textContent = error.message; }
    finally { setBusy(button, false); refreshControls(); }
  }

  function exportNetworkResult() {
    if (!state.networkInvestigation) return; const blob = new Blob([JSON.stringify(state.networkInvestigation, null, 2)], { type: 'application/json' }); const url = URL.createObjectURL(blob); const link = document.createElement('a');
    const safeId = String(state.networkInvestigation.id || 'network-result').replace(/[^a-zA-Z0-9._-]/g, '_'); link.href = url; link.download = `network-investigation-${safeId}.json`; document.body.append(link); link.click(); link.remove(); URL.revokeObjectURL(url);
  }

  function exportExplorationResult() {
    if (!state.networkExploration) return; const blob = new Blob([JSON.stringify(state.networkExploration, null, 2)], { type: 'application/json' }); const url = URL.createObjectURL(blob); const link = document.createElement('a'); const safeId = String(state.networkExploration.id || 'network-exploration').replace(/[^a-zA-Z0-9._-]/g, '_'); link.href = url; link.download = `network-exploration-${safeId}.json`; document.body.append(link); link.click(); link.remove(); URL.revokeObjectURL(url);
  }

  function renderExplorationHistory(items) {
    const list = $('exploration-history-list'); list.replaceChildren(); if (!items.length) { list.append(el('div', 'history-empty', 'No saved branch explorations yet.')); return; }
    items.slice(0, 8).forEach((item) => { const row = el('div', 'history-row'); const disposition = String(item.disposition || 'indeterminate').replaceAll('_', '-'); row.append(el('span', `history-decision path-history-decision path-${disposition}`, String(item.disposition || '?').slice(0, 1).toUpperCase())); const details = el('div', 'history-details'); details.append(el('strong', '', `${item.source && item.source.node_id || '?'} → ${item.destination && item.destination.node_id || '?'} · ${item.node_count || 0} devices`)); details.append(el('span', '', `${item.created_at ? new Date(item.created_at).toLocaleString() : 'Saved'} · ${item.branch_count || 0} branches · ${item.disposition || 'indeterminate'}`)); const open = el('button', 'history-open exploration-history-open', 'Open →'); open.type = 'button'; open.addEventListener('click', () => reopenNetworkExploration(item.id)); row.append(details, open); list.append(row); });
  }

  async function loadExplorationHistory() { const body = await request('/network-explorations'); renderExplorationHistory(Array.isArray(body.explorations) ? body.explorations : []); }

  function applyNetworkRequest(req) {
    $('network-nodes').replaceChildren(); $('network-links').replaceChildren(); (req.nodes || []).forEach(addNetworkNode); (req.links || []).forEach(addNetworkLink); refreshNetworkTopology();
    (req.nodes || []).forEach((node, index) => { const row = networkNodeRows()[index]; row.querySelector('.network-node-id').value = node.node_id; row.querySelector('.network-snapshot-id').value = node.snapshot_id; });
    (req.links || []).forEach((link, index) => { const row = document.querySelectorAll('.network-link-row')[index]; row.querySelector('.network-link-a-node').value = link.a.node_id; row.querySelector('.network-link-a-interface').value = link.a.interface; row.querySelector('.network-link-b-node').value = link.b.node_id; row.querySelector('.network-link-b-interface').value = link.b.interface; });
    refreshNetworkTopology(); $('network-source-node').value = req.source.node_id; $('network-source-interface').value = req.source.interface; $('network-destination-node').value = req.destination.node_id; $('network-destination-interface').value = req.destination.interface; $('network-flow-source').value = req.flow.source; $('network-flow-destination').value = req.flow.destination; $('network-flow-protocol').value = req.flow.protocol; $('network-flow-source-port').value = req.flow.source_port || ''; $('network-flow-destination-port').value = req.flow.destination_port || '';
  }

  async function reopenNetworkExploration(id) {
    if (state.busy) return; const before = JSON.stringify(networkPayload()); state.busy = true; refreshControls();
    try { const body = await request(`/network-explorations/${encodeURIComponent(id)}`); const exploration = body.exploration; if (!exploration || !exploration.request) throw new Error('Saved network exploration was incomplete.'); await Promise.all(exploration.request.nodes.map((node) => networkSnapshot(node.snapshot_id))); if (JSON.stringify(networkPayload()) !== before) throw new Error('Network topology or flow changed while history was loading. The saved exploration was not displayed.'); applyNetworkRequest(exploration.request); renderNetworkExploration(exploration); $('exploration-result').scrollIntoView({ behavior: 'smooth', block: 'nearest' }); }
    catch (error) { showToast(error.message, 'error'); } finally { state.busy = false; refreshControls(); }
  }

  function showEvidence(item, trigger = document.activeElement, snapshot = state.snapshot) {
    if (!snapshot || !item) return;
    state.evidenceTrigger = trigger && trigger.isConnected ? trigger : null;
    const lines = snapshot.source_lines || [];
    const from = Math.max(1, Number(item.start_line) || 1);
    const to = Math.max(from, Number(item.end_line) || from);
    const selected = lines.filter((line) => line.number >= from && line.number <= to);
    const pre = $('source-text');
    const pageSize = 200;
    const pageCount = Math.max(1, Math.ceil(selected.length / pageSize));
    let page = 0;
    const pager = $('source-pagination'); pager.replaceChildren();
    const previous = el('button', 'text-button', 'Previous lines'); previous.type = 'button'; previous.id = 'source-page-prev';
    const next = el('button', 'text-button', 'Next lines'); next.type = 'button'; next.id = 'source-page-next';
    if (pageCount > 1) pager.append(previous, document.createTextNode(' · '), next);
    const drawPage = () => {
      pre.replaceChildren();
      const pageLines = selected.slice(page * pageSize, (page + 1) * pageSize);
      pageLines.forEach((line) => {
        const row = el('span', 'source-row');
        const num = el('span', 'line-number', String(line.number).padStart(4, ' '));
        const text = el('span', 'line-content', line.text);
        row.append(num, text); pre.append(row, document.createTextNode('\n'));
      });
      $('source-caption').textContent = `${snapshot.hostname || snapshot.name || 'Snapshot'} · ${snapshot.id ? snapshot.id.slice(0, 12) : ''} · ${item.kind || 'SOURCE'} · SPAN ${from}–${to}${pageCount > 1 ? ` · page ${page + 1} of ${pageCount} (up to 200 lines)` : ''}`;
      if (!pageLines.length) pre.textContent = `Source lines ${from}–${to} are unavailable in this snapshot.`;
      previous.disabled = page === 0; next.disabled = page >= pageCount - 1;
    };
    previous.addEventListener('click', () => { if (page > 0) { page--; drawPage(); pre.focus({ preventScroll: true }); } });
    next.addEventListener('click', () => { if (page < pageCount - 1) { page++; drawPage(); pre.focus({ preventScroll: true }); } });
    drawPage();
    $('source-lines').hidden = false;
    pre.focus({ preventScroll: true });
    $('source-lines').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  function evidenceLink(item) {
    const button = el('button', 'evidence-link');
    button.type = 'button'; button.textContent = `${item.kind || 'Source'} · lines ${item.start_line}–${item.end_line}`;
    button.title = item.summary || item.id;
    button.addEventListener('click', (event) => showEvidence(item, event.currentTarget));
    return button;
  }

  function renderRuleEvidence(rule) {
    const wrap = el('div', 'rule-evidence');
    const refs = rule.evidence_refs || [];
    const items = evidenceForRefs(refs);
    if (items.length) items.forEach((item) => wrap.append(evidenceLink(item)));
    else if (refs.length) refs.forEach((ref) => wrap.append(el('span', 'ref-unresolved', `Evidence ${String(ref).slice(0, 22)}`)));
    else wrap.append(el('span', 'no-evidence', '—'));
    return wrap;
  }

  function renderTrace(investigation, page = 0) {
    const result = investigation.result;
    const chain = (state.snapshot.chains || []).find((c) => c.id === investigation.chain_id);
    const rules = chain ? chain.rules || [] : [];
    const considered = new Set(result.considered_rules || []);
    const traces = new Map((result.rule_traces || []).map((trace) => [trace.rule_id, trace]));
    const included = rules.filter((rule) => considered.has(rule.id));
    const pageSize = 100;
    const pageCount = Math.max(1, Math.ceil(included.length / pageSize));
    page = Math.max(0, Math.min(page, pageCount - 1));
    const visible = included.slice(page * pageSize, (page + 1) * pageSize);
    const tbody = $('trace-body'); tbody.replaceChildren();
    $('trace-count').textContent = `${included.length} rule${included.length === 1 ? '' : 's'}${pageCount > 1 ? ` · page ${page + 1} of ${pageCount}` : ''}`;
    $('trace-empty').hidden = included.length !== 0;
    let pager = $('trace-pagination');
    if (!pager) {
      pager = el('div', 'pagination-controls'); pager.id = 'trace-pagination'; pager.setAttribute('aria-label', 'Considered rule trace pages');
      $('predicate-trace').before(pager);
    }
    pager.replaceChildren(); pager.hidden = pageCount === 1;
    if (pageCount > 1) {
      const previous = el('button', 'button button-secondary button-small', '← Previous'); previous.type = 'button'; previous.id = 'trace-prev'; previous.disabled = page === 0;
      const next = el('button', 'button button-secondary button-small', 'Next →'); next.type = 'button'; next.id = 'trace-next'; next.disabled = page >= pageCount - 1;
      previous.addEventListener('click', () => { if (page > 0) renderTrace(investigation, page - 1); });
      next.addEventListener('click', () => { if (page < pageCount - 1) renderTrace(investigation, page + 1); });
      pager.append(previous, el('span', '', `Page ${page + 1} of ${pageCount}`), next);
    }
    visible.forEach((rule) => {
      const tr = el('tr', 'trace-row');
      const decisive = rule.id === result.decisive_rule;
      const trace = traces.get(rule.id);
      if (decisive) tr.classList.add('is-decisive');
      if (trace && trace.outcome === 'unresolved') tr.classList.add('is-candidate');
      const traceState = resultTraceState(result);
      const status = trace ? `${trace.outcome.replaceAll('_', ' ')}${decisive ? ` · ${result.disposition}` : ''}` : traceState === 'legacy' ? 'Legacy result · trace unavailable' : traceState === 'empty' ? 'No enabled rules were considered' : 'Saved result · trace unavailable';
      const statusClass = trace && trace.outcome === 'matched' ? `trace-status ${rule.action === 'deny' ? 'deny' : 'permit'}` : trace && trace.outcome === 'unresolved' ? 'trace-status possible' : 'trace-status neutral';
      const statusCell = el('td'); statusCell.append(el('span', statusClass, status));
      const ruleCell = el('td'); const ruleId = el('strong', 'rule-id', rule.id); ruleId.title = rule.id; ruleCell.append(ruleId);
      if (decisive) ruleCell.append(el('span', 'trace-note', 'Decisive rule'));
      else if (trace && trace.outcome === 'unresolved') ruleCell.append(el('span', 'trace-note', 'Evaluator marked unresolved'));
      const action = el('td'); action.append(el('span', `action-tag ${rule.action === 'deny' ? 'action-deny' : 'action-permit'}`, rule.action || '—'));
      const source = el('td', 'predicate-cell', rule.source || '—'); const destination = el('td', 'predicate-cell', rule.destination || '—');
      const serviceText = [rule.protocol || 'any', rule.destination_port ? `· ${rule.destination_port}` : ''].filter(Boolean).join(' ');
      const service = el('td', 'predicate-cell', serviceText);
      const evidence = el('td'); evidence.append(renderRuleEvidence(rule));
      tr.append(statusCell, ruleCell, action, source, destination, service, evidence);
      tr.tabIndex = 0; tr.setAttribute('aria-label', `${status}. Rule ${rule.id}, ${rule.action}.`);
      const selectEvidence = () => {
        const items = evidenceForRefs(rule.evidence_refs || []);
        renderEvidenceCards(items, rule);
        renderPredicateTrace(rule.id);
      };
      tr.addEventListener('click', (event) => { if (!event.target.closest('button')) selectEvidence(); });
      tr.addEventListener('keydown', (event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectEvidence(); } });
      tbody.append(tr);
    });
    const refs = result.evidence_refs || [];
    const resultEvidence = evidenceForRefs(refs);
    renderEvidenceCards(resultEvidence, null);
    renderPredicateTrace(visible.length ? visible[visible.length - 1].id : '');
  }

  function renderEvidenceCards(items, rule) {
    const panel = $('evidence-cards'); panel.replaceChildren();
    if (rule) panel.append(el('div', 'evidence-context', `Rule ${rule.id} · ${rule.action || 'action unavailable'}`));
    if (!items.length) {
      const chain = selectedChain();
      const refs = rule ? rule.evidence_refs || [] : (state.investigation && state.investigation.result.evidence_refs) || (chain && chain.evidence_refs) || [];
      panel.append(el('div', 'evidence-placeholder', refs.length ? 'Evidence reference is present, but no source locator is available.' : 'No source evidence was attached to this result.'));
      return;
    }
    items.forEach((item) => {
      const card = el('div', 'evidence-card');
      const head = el('div', 'evidence-card-head');
      head.append(el('span', 'evidence-type', item.kind || 'Configuration evidence'));
      const lines = el('span', 'line-range', `L${item.start_line}–${item.end_line}`); head.append(lines);
      card.append(head, el('p', 'evidence-summary', item.summary || 'Source evidence'));
      const open = el('button', 'open-evidence', 'Open source lines ↗'); open.type = 'button'; open.addEventListener('click', (event) => showEvidence(item, event.currentTarget)); card.append(open);
      panel.append(card);
    });
  }

  function renderStringList(id, values, fallback) {
    const list = $(id); list.replaceChildren();
    const items = Array.isArray(values) && values.length ? values : [fallback];
    items.forEach((value) => list.append(el('li', '', value)));
  }

  function renderInvestigation(investigation) {
    const result = investigation && investigation.result;
    const chain = state.snapshot && (state.snapshot.chains || []).find((item) => item.id === investigation.chain_id);
    if (!result || !state.snapshot || state.snapshot.id !== investigation.snapshot_id || !chain) {
      showToast('The saved result does not match the active snapshot and chain. It was not displayed.', 'error');
      return false;
    }
    state.investigation = investigation;
    $('result-empty').hidden = true;
    $('result-content').hidden = false;
    $('export-button').disabled = state.busy;
    $('result-time').textContent = investigation.created_at ? new Date(investigation.created_at).toLocaleString() : '';
    const disposition = String(result.disposition || 'indeterminate').toLowerCase();
    const banner = $('decision-banner');
    banner.className = `decision-banner decision-${disposition}`;
    $('decision').textContent = disposition.toUpperCase();
    $('decision-kicker').textContent = disposition === 'indeterminate' ? 'INCOMPLETE EVIDENCE' : 'POLICY DECISION';
    const ruleId = result.decisive_rule || result.candidate_rule;
    $('decision-detail').textContent = ruleId ? `${result.decisive_rule ? 'First matching rule' : 'Possible candidate'} · ${ruleId}` : result.used_default ? 'Explicit default action' : 'No definitive rule established';
    $('decision-symbol').textContent = disposition === 'permit' ? '✓' : disposition === 'deny' ? '×' : '?';
    const flow = result.flow || {};
    const protocol = String(flow.protocol || 'protocol unavailable').toUpperCase();
    const flowPorts = (protocol === 'TCP' || protocol === 'UDP') ? ` · ${flow.source_port} → ${flow.destination_port}` : '';
    $('result-flow').textContent = `${flow.source || 'Source unavailable'} → ${flow.destination || 'destination unavailable'} · ${protocol}${flowPorts}`;
    $('trace-subtitle').textContent = `${state.snapshot.hostname || 'ASA device'} · ${chain.name || investigation.chain_id} · ${investigation.snapshot_id}`;
    renderTrace(investigation);
    renderInventory();
    renderStringList('reason-list', result.reasons, 'No additional explanation was returned.');
    renderStringList('unsupported-list', result.unsupported_dimensions, 'No unsupported condition labels are reported for this result. Review chain coverage and rule explanations.');
    renderStringList('assumption-list', result.assumptions, 'Policy result applies to the selected chain only.');
    $('result-content').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    return true;
  }

  function renderHistory(items) {
    const list = $('history-list'); list.replaceChildren();
    if (!items.length) { list.append(el('div', 'history-empty', 'No saved investigations yet.')); return; }
    items.slice(0, 8).forEach((item) => {
      const result = item.result || {};
      const row = el('div', 'history-row');
      const icon = el('span', `history-decision ${String(result.disposition || 'indeterminate')}`, String(result.disposition || '?').slice(0, 1).toUpperCase());
      const details = el('div', 'history-details');
      details.append(el('strong', '', `${item.chain_id || 'ACL chain'} · ${result.flow ? `${result.flow.source} → ${result.flow.destination}` : 'Flow result'}`));
      details.append(el('span', '', `${item.created_at ? new Date(item.created_at).toLocaleString() : 'Saved'} · snapshot ${item.snapshot_id || 'unknown'}`));
      const action = el('button', 'history-open', 'Open result →'); action.type = 'button'; action.setAttribute('aria-label', `Open ${result.disposition || 'saved'} investigation for ${item.chain_id || 'chain'}`);
      action.addEventListener('click', () => reopenInvestigation(item.id));
      row.append(icon, details, action); list.append(row);
    });
  }

  async function loadSnapshot(id, preserveChain = '') {
    if (!id) { state.snapshot = null; state.pathInvestigation = null; state.selectedObjectID = ''; updateScope(); renderPathAvailability(null); clearResult(); refreshControls(); return; }
    const requestId = ++state.scopeRequest;
    const wasBusy = state.busy;
    state.busy = true;
    refreshControls();
    try {
      const body = await request(`/snapshots/${encodeURIComponent(id)}`);
      if (requestId !== state.scopeRequest) return false;
      if (!body.snapshot) throw new Error('Snapshot response was incomplete. Refresh the workspace.');
      setCurrentSnapshot(body.snapshot, preserveChain);
      $('snapshot-select').value = id;
      return true;
    } catch (error) { showToast(error.message, 'error'); }
    finally {
      state.busy = wasBusy;
      if (state.snapshot) $('snapshot-select').value = state.snapshot.id;
      else $('snapshot-select').value = '';
      refreshControls();
    }
  }

  async function loadSnapshots(preferredId = '') {
    const body = await request('/snapshots');
    const snapshots = Array.isArray(body.snapshots) ? body.snapshots : [];
    state.snapshots = snapshots;
    const select = $('snapshot-select'); select.replaceChildren();
    if (!snapshots.length) {
      select.add(new Option('No snapshots', ''));
      state.snapshot = null; state.pathInvestigation = null; updateScope(); renderPathAvailability(null); clearResult(); refreshControls();
      return;
    }
    select.add(new Option('Select a snapshot', ''));
    snapshots.forEach((snapshot) => select.add(new Option(summaryName(snapshot), snapshot.id)));
    const current = snapshots.find((item) => item.id === (preferredId || (state.snapshot && state.snapshot.id)));
    if (current) {
      select.value = current.id;
      await loadSnapshot(current.id);
    } else {
      select.value = '';
      state.snapshot = null; state.pathInvestigation = null; updateScope(); renderPathAvailability(null); clearResult(); refreshControls();
    }
  }

  async function loadHistory() {
    const body = await request('/investigations');
    const items = Array.isArray(body.investigations) ? body.investigations : [];
    items.sort((a, b) => String(b.created_at || '').localeCompare(String(a.created_at || '')));
    renderHistory(items);
    await loadPathHistory();
  }

  async function refreshWorkspace() {
    $('refresh-button').classList.add('rotating');
    const wasBusy = state.busy;
    state.busy = true; refreshControls();
    try { await loadSnapshots(state.snapshot && state.snapshot.id); await loadHistory(); }
    catch (error) { showToast(error.message, 'error'); }
    finally { state.busy = wasBusy; refreshControls(); window.setTimeout(() => $('refresh-button').classList.remove('rotating'), 400); }
  }

  async function loadSample() {
    const button = $('load-sample'); button.disabled = true;
    try {
      const body = await request('/sample');
      $('config-input').value = body.config || '';
      $('config-vendor').value = 'cisco-asa';
      $('config-meta').textContent = body.name ? `Labeled sample · ${body.name}` : 'Labeled ASA sample loaded';
      $('config-meta').dataset.name = body.name || 'labeled-asa-sample.txt';
      $('file-input').value = '';
      refreshControls(); showToast('Labeled ASA sample loaded. Import it to parse real configuration syntax.');
      $('config-input').focus();
    } catch (error) { showToast(error.message, 'error'); }
    finally { button.disabled = false; }
  }

  async function loadServiceSample() {
    const button = $('load-service-sample'); button.disabled = true;
    try {
      const body = await request('/sample/services');
      $('config-input').value = body.config || '';
      $('config-vendor').value = 'cisco-asa';
      $('config-meta').textContent = body.name ? `Service-group sample · ${body.name}` : 'Labeled service-group sample loaded';
      $('config-meta').dataset.name = body.name || 'service-group-sample.txt';
      $('file-input').value = '';
      refreshControls(); showToast('Service-group sample loaded. Import it to inspect bounded object expansion.');
      $('config-input').focus();
    } catch (error) { showToast(error.message, 'error'); }
    finally { button.disabled = false; }
  }

  async function loadVendorSample(kind) {
    const button = $(`load-${kind}-sample`); button.disabled = true;
    try { const sample = await request(`/sample/${kind}`); $('config-input').value = sample.config || ''; $('config-vendor').value = sample.vendor; $('config-meta').textContent = `${vendorName(sample.vendor)} sample · ${sample.name}`; $('config-meta').dataset.name = sample.name; $('file-input').value = ''; refreshControls(); }
    catch (error) { showToast(error.message, 'error'); } finally { button.disabled = false; }
  }

  async function importConfig() {
    const config = $('config-input').value;
    if (!config.trim()) return;
    const button = $('import-button'); setBusy(button, true, 'Parsing configuration…');
    try {
      const name = $('config-meta').dataset.name || 'pasted-firewall-config.txt';
      const body = await request('/imports', { method: 'POST', body: JSON.stringify({ name, config, vendor: $('config-vendor').value }) });
      const snapshot = body.snapshot;
      if (!snapshot || !snapshot.id) throw new Error('Import response did not include a snapshot. Refresh and try again.');
      const existing = state.snapshots.some((item) => item.id === snapshot.id);
      if (!existing) state.snapshots.unshift(snapshot);
      await loadSnapshots(snapshot.id);
      await loadHistory();
      showToast('Configuration imported and snapshot selected.');
    } catch (error) { showToast(error.message, 'error'); }
    finally { setBusy(button, false); refreshControls(); }
  }

  function flowPayload() {
    const protocol = $('flow-protocol').value;
    const hasPorts = protocol === 'tcp' || protocol === 'udp';
    const parsePort = (id) => {
      const value = Number($(id).value);
      return Number.isInteger(value) ? value : 0;
    };
    return {
      chain_id: $('chain-select').value,
      source: $('flow-source').value.trim(),
      destination: $('flow-destination').value.trim(),
      protocol,
      source_port: hasPorts ? parsePort('flow-source-port') : 0,
      destination_port: hasPorts ? parsePort('flow-destination-port') : 0
    };
  }

  async function evaluateFlow() {
    const snapshot = state.snapshot; const chain = selectedChain();
    if (!snapshot || !chain) return;
    const payload = flowPayload();
    const button = $('evaluate-button'); setBusy(button, true, 'Evaluating…');
    try {
      const body = await request(`/snapshots/${encodeURIComponent(snapshot.id)}/evaluate`, { method: 'POST', body: JSON.stringify(payload) });
      if (state.snapshot !== snapshot || selectedChain() !== chain || !body.investigation || body.investigation.snapshot_id !== snapshot.id || body.investigation.chain_id !== chain.id) throw new Error('The evaluation response did not match the selected snapshot and chain. Refresh before continuing.');
      const evaluatedFlow = body.investigation.result && body.investigation.result.flow;
      if (!evaluatedFlow || evaluatedFlow.source !== payload.source || evaluatedFlow.destination !== payload.destination || evaluatedFlow.protocol !== payload.protocol || Number(evaluatedFlow.source_port || 0) !== payload.source_port || Number(evaluatedFlow.destination_port || 0) !== payload.destination_port) throw new Error('The evaluation response did not match the submitted flow. The result was not displayed.');
      renderInvestigation(body.investigation);
      await loadHistory();
    } catch (error) { showToast(error.message, 'error'); }
    finally { setBusy(button, false); refreshControls(); }
  }

  async function reopenInvestigation(id) {
    if (state.busy) return;
    state.busy = true; refreshControls();
    try {
      const body = await request(`/investigations/${encodeURIComponent(id)}`);
      const investigation = body.investigation;
      if (!investigation) throw new Error('Saved investigation was not returned.');
      await loadSnapshot(investigation.snapshot_id, investigation.chain_id);
      if (!state.snapshot || state.snapshot.id !== investigation.snapshot_id || !selectedChain() || selectedChain().id !== investigation.chain_id) throw new Error('Saved scope is unavailable; the result was not opened in a different scope.');
      $('snapshot-select').value = investigation.snapshot_id;
      const flow = investigation.result && investigation.result.flow;
      if (!flow || !renderInvestigation(investigation)) throw new Error('Saved flow details were missing or did not match its scope.');
      $('flow-source').value = flow.source;
      $('flow-destination').value = flow.destination;
      $('flow-protocol').value = flow.protocol;
      const hasPorts = flow.protocol === 'tcp' || flow.protocol === 'udp';
      $('flow-source-port').disabled = !hasPorts;
      $('flow-destination-port').disabled = !hasPorts;
      $('flow-source-port').value = hasPorts ? String(flow.source_port) : '';
      $('flow-destination-port').value = hasPorts ? String(flow.destination_port) : '';
    } catch (error) { showToast(error.message, 'error'); }
    finally { state.busy = false; refreshControls(); }
  }

  function exportResult() {
    if (!state.investigation) return;
    const blob = new Blob([JSON.stringify(state.investigation, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    const safeId = String(state.investigation.id || 'result').replace(/[^a-zA-Z0-9._-]/g, '_');
    link.href = url; link.download = `investigation-${safeId}.json`;
    document.body.append(link); link.click(); link.remove(); URL.revokeObjectURL(url);
  }

  function exportPathResult() {
    if (!state.pathInvestigation) return;
    const blob = new Blob([JSON.stringify(state.pathInvestigation, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    const safeId = String(state.pathInvestigation.id || 'path-result').replace(/[^a-zA-Z0-9._-]/g, '_');
    link.href = url; link.download = `path-investigation-${safeId}.json`;
    document.body.append(link); link.click(); link.remove(); URL.revokeObjectURL(url);
  }

  function wireEvents() {
    $('network-load-sample').addEventListener('click', () => loadNetworkSample());
    $('network-load-vendor-sample').addEventListener('click', () => loadNetworkSample('/sample/network-multivendor'));
    $('network-load-exploration-sample').addEventListener('click', () => loadNetworkSample('/sample/network-exploration'));
    $('load-panos-sample').addEventListener('click', () => loadVendorSample('panos'));
    $('load-fortigate-sample').addEventListener('click', () => loadVendorSample('fortigate'));
    $('network-add-node').addEventListener('click', () => addNetworkNode());
    $('network-add-link').addEventListener('click', () => addNetworkLink());
    $('network-evaluate').addEventListener('click', evaluateNetwork);
    $('network-explore').addEventListener('click', exploreNetwork);
    $('network-export').addEventListener('click', exportNetworkResult);
    $('exploration-export').addEventListener('click', exportExplorationResult);
    $('refresh-network-history').addEventListener('click', () => loadNetworkHistory().catch((error) => showToast(error.message, 'error')));
    $('refresh-exploration-history').addEventListener('click', () => loadExplorationHistory().catch((error) => showToast(error.message, 'error')));
    document.querySelectorAll('#network-source-interface,#network-destination-interface,#network-flow-source,#network-flow-destination,#network-flow-source-port,#network-flow-destination-port').forEach((node) => node.addEventListener('input', () => { clearNetworkResult(); refreshControls(); }));
    ['network-source-node','network-destination-node','network-flow-protocol'].forEach((id) => $(id).addEventListener('change', () => { const enabled = ['tcp','udp'].includes($('network-flow-protocol').value); ['network-flow-source-port','network-flow-destination-port'].forEach((port) => { $(port).disabled = !enabled; if (!enabled) $(port).value = ''; }); clearNetworkResult(); refreshControls(); }));
    $('config-input').addEventListener('input', () => { $('config-meta').textContent = `${$('config-input').value.length.toLocaleString()} characters`; $('config-meta').dataset.name = ''; refreshControls(); });
    $('file-input').addEventListener('change', async (event) => {
      const file = event.target.files && event.target.files[0];
      if (!file) return;
      if (file.size > 2 * 1024 * 1024) {
        showToast('The configuration exceeds the 2 MiB local import limit.', 'error');
        event.target.value = '';
        return;
      }
      try {
        $('config-input').value = await file.text();
        $('config-meta').textContent = `${file.name} · ${file.size.toLocaleString()} bytes`;
        $('config-meta').dataset.name = file.name;
        refreshControls(); $('config-input').focus();
      } catch (_) { showToast('The selected local file could not be read.', 'error'); }
    });
    $('load-sample').addEventListener('click', loadSample);
    $('load-service-sample').addEventListener('click', loadServiceSample);
    $('import-button').addEventListener('click', importConfig);
    $('evaluate-button').addEventListener('click', evaluateFlow);
    $('path-evaluate-button').addEventListener('click', evaluatePath);
    $('path-export-button').addEventListener('click', exportPathResult);
    $('path-ingress').addEventListener('change', () => { updatePathInterfaceMeta(); clearPathResult(); });
    $('path-protocol').addEventListener('change', () => {
      const enabled = $('path-protocol').value === 'tcp' || $('path-protocol').value === 'udp';
      ['path-source-port', 'path-destination-port'].forEach((id) => { $(id).disabled = !enabled; if (!enabled) $(id).value = ''; else if (!$(id).value) $(id).value = id === 'path-source-port' ? '52000' : '443'; });
      clearPathResult();
    });
    ['path-source', 'path-destination', 'path-source-port', 'path-destination-port'].forEach((id) => $(id).addEventListener('input', clearPathResult));
    $('chain-select').addEventListener('change', () => { state.inventoryPage = 0; state.selectedRuleID = ''; updateChainMeta(); clearResult(); renderInventory(); refreshControls(); });
    ['inventory-search', 'inventory-action', 'inventory-status', 'inventory-resolution'].forEach((id) => $(id).addEventListener(id === 'inventory-search' ? 'input' : 'change', () => { state.inventoryPage = 0; renderInventory(); }));
    $('inventory-prev').addEventListener('click', () => { state.inventoryPage = Math.max(0, state.inventoryPage - 1); renderInventory(); });
    $('inventory-next').addEventListener('click', () => { state.inventoryPage += 1; renderInventory(); });
    $('object-search').addEventListener('input', () => { state.objectPage = 0; renderObjects(); });
    $('object-prev').addEventListener('click', () => { state.objectPage = Math.max(0, state.objectPage - 1); renderObjects(); });
    $('object-next').addEventListener('click', () => { state.objectPage += 1; renderObjects(); });
    $('snapshot-select').addEventListener('change', (event) => loadSnapshot(event.target.value));
    $('refresh-button').addEventListener('click', refreshWorkspace);
    $('refresh-history').addEventListener('click', () => loadHistory().catch((error) => showToast(error.message, 'error')));
    $('refresh-path-history').addEventListener('click', () => loadPathHistory().catch((error) => showToast(error.message, 'error')));
    $('export-button').addEventListener('click', exportResult);
    $('close-source').addEventListener('click', () => {
      $('source-lines').hidden = true;
      const trigger = state.evidenceTrigger;
      state.evidenceTrigger = null;
      if (trigger && trigger.isConnected && !trigger.closest('[hidden]')) trigger.focus();
      else $('inventory-search').focus();
    });
    $('flow-protocol').addEventListener('change', () => {
      const enabled = $('flow-protocol').value === 'tcp' || $('flow-protocol').value === 'udp';
      ['flow-source-port', 'flow-destination-port'].forEach((id) => { $(id).disabled = !enabled; if (!enabled) $(id).value = ''; else if (!$(id).value) $(id).value = id === 'flow-source-port' ? '52000' : '443'; });
      clearResult();
    });
    ['flow-source', 'flow-destination', 'flow-source-port', 'flow-destination-port'].forEach((id) => $(id).addEventListener('input', () => clearResult()));
    document.querySelectorAll('.nav-item').forEach((link) => link.addEventListener('click', (event) => {
      const target = link.getAttribute('href');
      if (target === '#imports') { event.preventDefault(); $('config-input').focus(); $('imports').scrollIntoView({ behavior: 'smooth' }); }
      else if (target === '#policy-inventory' || target === '#objects') { event.preventDefault(); const section = $(target.slice(1)); section.scrollIntoView({ behavior: 'smooth' }); (target === '#policy-inventory' ? $('inventory-search') : $('object-search')).focus({ preventScroll: true }); }
      else if (target === '#investigation') { event.preventDefault(); $('flow-source').focus(); }
    }));
  }

  async function start() {
    wireEvents(); addNetworkNode({ node_id: 'firewall-1' }); addNetworkNode({ node_id: 'firewall-2' }); refreshControls();
    try {
      const session = await request('/session');
      state.csrf = session.csrf_token || '';
      if (!state.csrf) throw new Error('The local session could not be initialized. Reload the page.');
      await loadSnapshots();
      await loadHistory();
      await loadNetworkHistory();
      await loadExplorationHistory();
    } catch (error) { showToast(error.message, 'error'); }
  }

  document.addEventListener('DOMContentLoaded', start);
})();
