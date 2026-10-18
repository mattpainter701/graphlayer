(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const state = { csrf: '', snapshots: [], snapshot: null, investigation: null, busy: false, scopeRequest: 0, toastTimer: 0 };
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
        invalid_import: 'Provide an ASA configuration and a display name of at most 128 characters.',
        import_failed: 'The configuration could not be imported as a supported ASA policy snapshot. Earlier snapshots are unchanged.',
        config_too_large: 'The configuration exceeds the 2 MiB local import limit.',
        input_too_large: 'The request exceeds the local import size limit.',
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
    return [snapshot.hostname || snapshot.name || 'ASA device', snapshot.context || 'default', snapshot.created_at ? new Date(snapshot.created_at).toLocaleString() : snapshot.id].filter(Boolean).join(' · ');
  }

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
    ['config-input', 'file-input', 'load-sample', 'flow-source', 'flow-destination', 'flow-protocol', 'refresh-button', 'refresh-history'].forEach((id) => { $(id).disabled = state.busy; });
    const usesPorts = $('flow-protocol').value === 'tcp' || $('flow-protocol').value === 'udp';
    $('flow-source-port').disabled = state.busy || !usesPorts;
    $('flow-destination-port').disabled = state.busy || !usesPorts;
    $('export-button').disabled = state.busy || !state.investigation;
    document.querySelectorAll('.history-open').forEach((button) => { button.disabled = state.busy; });
  }

  function setCurrentSnapshot(snapshot, preserveChain = '') {
    state.snapshot = snapshot;
    state.investigation = null;
    updateScope();
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
    refreshControls();
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
  }

  function normalizeRef(ref) { return String(ref); }

  function evidenceForRefs(refs) {
    const evidence = (state.snapshot && state.snapshot.evidence) || [];
    const wanted = new Set((refs || []).map(normalizeRef));
    return evidence.filter((item) => wanted.has(normalizeRef(item.id)));
  }

  function showEvidence(item) {
    if (!state.snapshot || !item) return;
    const lines = state.snapshot.source_lines || [];
    const from = Math.max(1, Number(item.start_line) || 1);
    const to = Math.max(from, Number(item.end_line) || from);
    const selected = lines.filter((line) => line.number >= from && line.number <= to);
    $('source-caption').textContent = `${item.kind || 'SOURCE'} · LINE${from === to ? '' : 'S'} ${from}${from === to ? '' : `–${to}`}`;
    const pre = $('source-text');
    pre.replaceChildren();
    selected.forEach((line) => {
      const row = el('span', 'source-row');
      const num = el('span', 'line-number', String(line.number).padStart(4, ' '));
      const text = el('span', 'line-content', line.text);
      row.append(num, text); pre.append(row, document.createTextNode('\n'));
    });
    if (!selected.length) pre.textContent = `Source lines ${from}–${to} are unavailable in this snapshot.`;
    $('source-lines').hidden = false;
    pre.focus({ preventScroll: true });
    $('source-lines').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  function evidenceLink(item) {
    const button = el('button', 'evidence-link');
    button.type = 'button'; button.textContent = `${item.kind || 'Source'} · lines ${item.start_line}–${item.end_line}`;
    button.title = item.summary || item.id;
    button.addEventListener('click', () => showEvidence(item));
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

  function renderTrace(investigation) {
    const result = investigation.result;
    const chain = (state.snapshot.chains || []).find((c) => c.id === investigation.chain_id);
    const rules = chain ? chain.rules || [] : [];
    const considered = new Set(result.considered_rules || []);
    const included = rules.filter((rule) => considered.has(rule.id));
    const tbody = $('trace-body'); tbody.replaceChildren();
    $('trace-count').textContent = `${included.length} rule${included.length === 1 ? '' : 's'}`;
    $('trace-empty').hidden = included.length !== 0;
    included.forEach((rule) => {
      const tr = el('tr', 'trace-row');
      const decisive = rule.id === result.decisive_rule;
      const candidate = rule.id === result.candidate_rule;
      const possible = result.disposition === 'indeterminate' && !decisive && !candidate && (rule.unsupported || []).length > 0;
      if (decisive) tr.classList.add('is-decisive');
      if (candidate) tr.classList.add('is-candidate');
      const status = decisive ? (result.disposition === 'deny' ? 'Matched · deny' : 'Matched · permit') : candidate || possible ? 'Possible match' : 'Evaluated';
      const statusClass = decisive ? (result.disposition === 'deny' ? 'trace-status deny' : 'trace-status permit') : candidate || possible ? 'trace-status possible' : 'trace-status neutral';
      const statusCell = el('td'); statusCell.append(el('span', statusClass, status));
      const ruleCell = el('td'); const ruleId = el('strong', 'rule-id', rule.id); ruleId.title = rule.id; ruleCell.append(ruleId);
      if (decisive) ruleCell.append(el('span', 'trace-note', 'Decisive rule'));
      else if (candidate) ruleCell.append(el('span', 'trace-note', 'Candidate rule'));
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
      };
      tr.addEventListener('click', (event) => { if (!event.target.closest('button')) selectEvidence(); });
      tr.addEventListener('keydown', (event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectEvidence(); } });
      tbody.append(tr);
    });
    const refs = result.evidence_refs || [];
    const resultEvidence = evidenceForRefs(refs);
    renderEvidenceCards(resultEvidence, null);
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
      const open = el('button', 'open-evidence', 'Open source lines ↗'); open.type = 'button'; open.addEventListener('click', () => showEvidence(item)); card.append(open);
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
    renderStringList('reason-list', result.reasons, 'No additional explanation was returned.');
    renderStringList('unsupported-list', result.unsupported_dimensions, 'No unsupported dimensions were encountered by the evaluator.');
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
    if (!id) { state.snapshot = null; updateScope(); clearResult(); refreshControls(); return; }
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
      state.snapshot = null; updateScope(); clearResult(); refreshControls();
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
      state.snapshot = null; updateScope(); clearResult(); refreshControls();
    }
  }

  async function loadHistory() {
    const body = await request('/investigations');
    const items = Array.isArray(body.investigations) ? body.investigations : [];
    items.sort((a, b) => String(b.created_at || '').localeCompare(String(a.created_at || '')));
    renderHistory(items);
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
      $('config-meta').textContent = body.name ? `Labeled sample · ${body.name}` : 'Labeled ASA sample loaded';
      $('config-meta').dataset.name = body.name || 'labeled-asa-sample.txt';
      $('file-input').value = '';
      refreshControls(); showToast('Labeled ASA sample loaded. Import it to parse real configuration syntax.');
      $('config-input').focus();
    } catch (error) { showToast(error.message, 'error'); }
    finally { button.disabled = false; }
  }

  async function importConfig() {
    const config = $('config-input').value;
    if (!config.trim()) return;
    const button = $('import-button'); setBusy(button, true, 'Parsing configuration…');
    try {
      const name = $('config-meta').dataset.name || 'pasted-asa-config.txt';
      const body = await request('/imports', { method: 'POST', body: JSON.stringify({ name, config }) });
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

  function wireEvents() {
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
    $('import-button').addEventListener('click', importConfig);
    $('evaluate-button').addEventListener('click', evaluateFlow);
    $('chain-select').addEventListener('change', () => { updateChainMeta(); clearResult(); refreshControls(); });
    $('snapshot-select').addEventListener('change', (event) => loadSnapshot(event.target.value));
    $('refresh-button').addEventListener('click', refreshWorkspace);
    $('refresh-history').addEventListener('click', () => loadHistory().catch((error) => showToast(error.message, 'error')));
    $('export-button').addEventListener('click', exportResult);
    $('close-source').addEventListener('click', () => { $('source-lines').hidden = true; $('evidence-panel').focus(); });
    $('flow-protocol').addEventListener('change', () => {
      const enabled = $('flow-protocol').value === 'tcp' || $('flow-protocol').value === 'udp';
      ['flow-source-port', 'flow-destination-port'].forEach((id) => { $(id).disabled = !enabled; if (!enabled) $(id).value = ''; else if (!$(id).value) $(id).value = id === 'flow-source-port' ? '52000' : '443'; });
      clearResult();
    });
    ['flow-source', 'flow-destination', 'flow-source-port', 'flow-destination-port'].forEach((id) => $(id).addEventListener('input', () => clearResult()));
    document.querySelectorAll('.nav-item').forEach((link) => link.addEventListener('click', (event) => {
      const target = link.getAttribute('href');
      if (target === '#imports') { event.preventDefault(); $('config-input').focus(); $('imports').scrollIntoView({ behavior: 'smooth' }); }
      else if (target === '#rules') { event.preventDefault(); if (state.snapshot && state.snapshot.chains && state.snapshot.chains.length) $('chain-select').focus(); else showToast('Import a configuration to inspect its policy chains.'); }
      else if (target === '#investigation') { event.preventDefault(); $('flow-source').focus(); }
    }));
  }

  async function start() {
    wireEvents(); refreshControls();
    try {
      const session = await request('/session');
      state.csrf = session.csrf_token || '';
      if (!state.csrf) throw new Error('The local session could not be initialized. Reload the page.');
      await loadSnapshots();
      await loadHistory();
    } catch (error) { showToast(error.message, 'error'); }
  }

  document.addEventListener('DOMContentLoaded', start);
})();
