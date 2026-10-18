const { test, expect } = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');

async function jsonFromPage(page, url) {
  return page.evaluate(async (path) => {
    const response = await fetch(path, { credentials: 'same-origin' });
    return { status: response.status, body: await response.json() };
  }, url);
}

function addressFor(predicate, fallback) {
  if (!predicate || predicate.toLowerCase() === 'any') return fallback;
  const value = predicate.trim();
  if (value.includes('/')) return value.split('/')[0];
  if (/^[0-9a-f:.]+$/i.test(value)) return value;
  return fallback;
}

function portFor(predicate, fallback) {
  if (!predicate) return fallback;
  const [low, high = low] = predicate.split('-').map(Number);
  return low === high ? low : Math.floor((low + high) / 2);
}

function flowFor(rule) {
  const normalizedProtocol = rule.protocol.toLowerCase();
  const protocol = ({ tcp: 'tcp', '6': 'tcp', udp: 'udp', '17': 'udp', icmp: 'icmp', '1': 'icmp', icmpv6: 'icmpv6', '58': 'icmpv6' })[normalizedProtocol] || 'tcp';
  const isPortProtocol = protocol === 'tcp' || protocol === 'udp' || protocol === '6' || protocol === '17';
  return {
    source: addressFor(rule.source, '198.51.100.42'),
    destination: addressFor(rule.destination, '203.0.113.10'),
    protocol,
    source_port: isPortProtocol ? portFor(rule.source_port, 49152) : 0,
    destination_port: isPortProtocol ? portFor(rule.destination_port, protocol === 'tcp' ? 443 : 53) : 0,
  };
}

async function importSample(page) {
  await page.goto('/#imports');
  await page.getByRole('button', { name: /load labeled asa sample/i }).click();
  await expect(page.locator('#config-input')).toHaveValue(/\S/);
  await page.getByRole('button', { name: /import configuration/i }).click();
  await expect(page.locator('#chain-select')).toBeEnabled();
  const summaries = await jsonFromPage(page, '/api/snapshots');
  expect(summaries.status).toBe(200);
  const snapshotId = summaries.body.snapshots[0].id;
  const detail = await jsonFromPage(page, `/api/snapshots/${encodeURIComponent(snapshotId)}`);
  expect(detail.status).toBe(200);
  return detail.body.snapshot;
}

async function evaluateRule(page, snapshot, rule, expected) {
  await page.locator('#chain-select').selectOption(snapshot.chains[0].id);
  const flow = flowFor(rule);
  await page.locator('#flow-source').fill(flow.source);
  await page.locator('#flow-destination').fill(flow.destination);
  await page.locator('#flow-protocol').selectOption(flow.protocol);
  await page.locator('#flow-source-port').fill(flow.source_port ? String(flow.source_port) : '');
  await page.locator('#flow-destination-port').fill(flow.destination_port ? String(flow.destination_port) : '');
  await page.getByRole('button', { name: /evaluate flow/i }).click();
  await expect(page.locator('#decision')).toContainText(expected, { timeout: 10_000 });
  return flow;
}

test('imports ASA syntax, traces permit/deny/indeterminate, and restores investigation history', async ({ page }, testInfo) => {
  const snapshot = await importSample(page);
  expect(snapshot.vendor.toLowerCase()).toContain('asa');
  expect(snapshot.chains.length).toBeGreaterThan(0);

  const rules = snapshot.chains[0].rules;
  const allow = rules.find((rule) => rule.action === 'permit' && rule.destination_port === '443');
  const deny = rules.find((rule) => rule.action === 'deny' && rule.destination_port === '22');
  const unresolved = rules.find((rule) => (rule.unsupported || []).length > 0 ||
    !['any'].includes(rule.source.toLowerCase()) && !/^[0-9a-f:.]+(?:\/\d+)?$/i.test(rule.source) ||
    !['any'].includes(rule.destination.toLowerCase()) && !/^[0-9a-f:.]+(?:\/\d+)?$/i.test(rule.destination));
  expect(allow, 'fixture has a parsed HTTPS permit ACE').toBeTruthy();
  expect(deny, 'fixture has a parsed SSH deny ACE').toBeTruthy();
  expect(unresolved, 'fixture retains an unresolved ACE').toBeTruthy();

  await evaluateRule(page, snapshot, allow, /permit/i);
  await expect(page.getByText('Policy only')).toBeVisible();
  await expect(page.getByText(/Routing, NAT & return path not evaluated/i)).toBeVisible();
  await expect(page.locator('#evidence-panel')).toBeVisible();
  await page.getByRole('button', { name: /open source lines/i }).first().click();
  await expect(page.locator('#source-lines')).toBeVisible();
  await expect(page.locator('#source-text')).toContainText(/access-list/i);
  const [exported] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: /export result/i }).click(),
  ]);
  expect(exported.suggestedFilename()).toMatch(/investigation.*\.json/i);
  const screenshotPath = path.resolve('test-results', 'investigation.png');
  await fs.mkdir(path.dirname(screenshotPath), { recursive: true });
  await page.screenshot({ path: screenshotPath, fullPage: true });
  await evaluateRule(page, snapshot, deny, /deny/i);
  const unresolvedFlow = await evaluateRule(page, snapshot, unresolved, /indeterminate/i);

  await page.reload();
  await expect(page.locator('#history-list').getByRole('button', { name: /open/i })).toHaveCount(3);
  await page.getByRole('button', { name: /open indeterminate investigation/i }).click();
  await expect(page.locator('#decision')).toContainText(/indeterminate/i);
  await expect(page.locator('#flow-source')).toHaveValue(unresolvedFlow.source);
  await expect(page.locator('#flow-destination')).toHaveValue(unresolvedFlow.destination);
  await expect(page.locator('#flow-protocol')).toHaveValue(unresolvedFlow.protocol);
  await expect(page.locator('#flow-source-port')).toHaveValue(unresolvedFlow.source_port ? String(unresolvedFlow.source_port) : '');
  await expect(page.locator('#flow-destination-port')).toHaveValue(unresolvedFlow.destination_port ? String(unresolvedFlow.destination_port) : '');
  await expect(page.locator('#result-flow')).toContainText(unresolvedFlow.source);
  await expect(page.locator('#result-flow')).toContainText(unresolvedFlow.destination);
  await page.locator('#flow-source').fill('192.0.2.99');
  await expect(page.locator('#result-content')).toBeHidden();
  await expect(page.locator('#export-button')).toBeDisabled();
  await page.screenshot({ path: testInfo.outputPath('firewall-investigation-e2e.png'), fullPage: true });
});

test('discards an evaluation response if its snapshot scope changes in flight', async ({ page }) => {
  const first = await importSample(page);
  await page.getByRole('button', { name: /load labeled asa sample/i }).click();
  const config = await page.locator('#config-input').inputValue();
  await page.locator('#config-input').fill(config.replace('hostname edge-fw', 'hostname edge-fw-second'));
  await page.getByRole('button', { name: /import configuration/i }).click();
  await expect(page.locator('#scope-device')).toContainText('edge-fw-second');
  const summaries = await jsonFromPage(page, '/api/snapshots');
  expect(summaries.body.snapshots.length).toBeGreaterThanOrEqual(2);
  const secondId = summaries.body.snapshots.find((item) => item.id !== first.id).id;
  const secondResponse = await jsonFromPage(page, `/api/snapshots/${encodeURIComponent(secondId)}`);
  const second = secondResponse.body.snapshot;

  await page.locator('#snapshot-select').selectOption(first.id);
  await expect(page.locator('#scope-device')).toContainText(first.hostname);
  await page.locator('#chain-select').selectOption(first.chains[0].id);
  const rule = first.chains[0].rules.find((candidate) => candidate.action === 'permit' && candidate.destination_port === '443');
  expect(rule).toBeTruthy();
  const flow = flowFor(rule);
  await page.locator('#flow-source').fill(flow.source);
  await page.locator('#flow-destination').fill(flow.destination);
  await page.locator('#flow-protocol').selectOption(flow.protocol);
  await page.locator('#flow-source-port').fill(String(flow.source_port));
  await page.locator('#flow-destination-port').fill(String(flow.destination_port));

  let releaseResponse;
  let signalRequest;
  const requestArrived = new Promise((resolve) => { signalRequest = resolve; });
  const responseReleased = new Promise((resolve) => { releaseResponse = resolve; });
  await page.route('**/api/snapshots/*/evaluate', async (route) => {
    const response = await route.fetch();
    signalRequest();
    await responseReleased;
    await route.fulfill({ response });
  });
  await page.getByRole('button', { name: /evaluate flow/i }).click();
  await requestArrived;
  await expect(page.locator('#snapshot-select')).toBeDisabled();
  await expect(page.locator('#chain-select')).toBeDisabled();
  await expect(page.locator('#flow-source')).toBeDisabled();

  // Simulate a late scope change even though the user-facing control is locked while busy.
  await page.locator('#snapshot-select').evaluate((select, id) => {
    select.disabled = false;
    select.value = id;
    select.dispatchEvent(new Event('change', { bubbles: true }));
  }, second.id);
  await expect(page.locator('#scope-device')).toContainText(second.hostname);
  releaseResponse();
  await expect(page.locator('#toast')).toContainText(/did not match the selected snapshot and chain/i);
  await expect(page.locator('#result-content')).toBeHidden();
  await expect(page.locator('#snapshot-select')).toHaveValue(second.id);
});

test('supports the core investigation flow at a mobile viewport without page-wide horizontal scrolling', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const snapshot = await importSample(page);
  const allow = snapshot.chains[0].rules.find((rule) => rule.action === 'permit' && rule.destination_port === '443');
  await evaluateRule(page, snapshot, allow, /permit/i);
  await expect(page.locator('#evidence-panel')).toBeVisible();
  const overflow = await page.evaluate(() => ({
    viewport: window.innerWidth,
    document: document.documentElement.scrollWidth,
    body: document.body.scrollWidth,
    offenders: [...document.querySelectorAll('body *')].map((element) => {
      const rect = element.getBoundingClientRect();
      return { selector: `${element.tagName.toLowerCase()}${element.id ? `#${element.id}` : ''}${element.classList && element.classList.length ? `.${[...element.classList].join('.')}` : ''}`, left: Math.round(rect.left), right: Math.round(rect.right), width: Math.round(rect.width), scrollWidth: element.scrollWidth };
    }).filter((item) => item.right > window.innerWidth + 1 || item.left < -1).sort((a, b) => b.right - a.right).slice(0, 20),
  }));
  expect(overflow.document, JSON.stringify(overflow)).toBeLessThanOrEqual(overflow.viewport);
  expect(overflow.body, JSON.stringify(overflow)).toBeLessThanOrEqual(overflow.viewport);
  await page.screenshot({ path: path.resolve('test-results', 'investigation-mobile.png'), fullPage: true });
});

test('failed import preserves the prior snapshot and source text is rendered inert', async ({ page }) => {
  const snapshot = await importSample(page);
  const before = await jsonFromPage(page, '/api/snapshots');

  await page.locator('#file-input').setInputFiles({
    name: 'malformed.asa.txt',
    mimeType: 'text/plain',
    buffer: Buffer.from('hostname "<img src=x onerror=alert(1)>"\nthis is not a valid ASA configuration'),
  });
  await expect(page.locator('#config-input')).toHaveValue(/this is not a valid ASA configuration/);
  await page.getByRole('button', { name: /import configuration/i }).click();
  await expect(page.locator('#toast')).toContainText(/could not be imported.*Earlier snapshots are unchanged/i);
  const after = await jsonFromPage(page, '/api/snapshots');
  expect(after.body.snapshots.map((row) => row.id)).toEqual(before.body.snapshots.map((row) => row.id));
  expect(await page.locator('img[src="x"]').count()).toBe(0);
  expect(await page.evaluate(() => window.__xssExecuted === true)).toBe(false);
  expect(snapshot.id).toBe(before.body.snapshots[0].id);
});

test('sanitized snapshots omit credential canaries and hostile source stays text', async ({ page }) => {
  await page.goto('/#imports');
  await page.getByRole('button', { name: /load labeled asa sample/i }).click();
  await expect(page.locator('#config-input')).toHaveValue(/\S/);
  const source = await page.locator('#config-input').inputValue();
  const hostileSource = source.replace(
    'access-group OUTSIDE-IN in interface outside',
    'access-group OUTSIDE-IN in interface outside ! <img src=x onerror=window.__xssExecuted=true>'
  );
  await page.locator('#config-input').fill(hostileSource);
  await page.getByRole('button', { name: /import configuration/i }).click();
  await expect(page.locator('#chain-select')).toBeEnabled();
  const summaries = await jsonFromPage(page, '/api/snapshots');
  const imported = await jsonFromPage(page, `/api/snapshots/${encodeURIComponent(summaries.body.snapshots[0].id)}`);
  const snapshot = imported.body.snapshot;
  expect(JSON.stringify(snapshot)).not.toMatch(/PASSWORD_CANARY|COMMUNITY_CANARY|TUNNEL_KEY_CANARY/);
  expect(snapshot.warnings.some((warning) => warning.code === 'unsupported_binding'), 'malformed access-group suffix is retained as an unsupported binding').toBe(true);
  await expect(page.locator('.warning-list')).toContainText(/unsupported access-group binding/i);
  await page.locator('#chain-select').selectOption(snapshot.chains[0].id);
  await expect(page.locator('#chain-meta')).toContainText(/partial chain/i);

  const rule = snapshot.chains[0].rules.find((candidate) => candidate.action === 'permit' && candidate.destination_port === '443');
  expect(rule).toBeTruthy();
  const hostileLine = snapshot.source_lines.find((line) => line.text.includes('<img src=x'));
  expect(hostileLine, 'hostile text is present in the sanitized, successfully imported source').toBeTruthy();
  const evidence = snapshot.evidence.find((item) => rule.evidence_refs.includes(item.id) &&
    item.start_line <= hostileLine.number && item.end_line >= hostileLine.number);
  expect(evidence, 'HTTPS rule evidence includes the referenced object description').toBeTruthy();
  await evaluateRule(page, snapshot, rule, /indeterminate/i);
  await page.locator('#trace-body tr').first().click();
  await page.locator('#evidence-cards .evidence-card')
    .filter({ hasText: `L${evidence.start_line}–${evidence.end_line}` })
    .getByRole('button', { name: /open source lines/i }).click();
  const sourceText = page.locator('#source-text');
  await expect(sourceText).toContainText('<img src=x');
  expect(await sourceText.locator('img').count()).toBe(0);
  expect(await page.locator('img[src="x"]').count()).toBe(0);
  expect(await page.evaluate(() => window.__xssExecuted === true)).toBe(false);
});

test('API rejects a foreign Origin even with an initialized browser session', async ({ page }) => {
  await page.goto('/#imports');
  const session = await jsonFromPage(page, '/api/session');
  expect(session.status).toBe(200);
  const response = await page.request.post('/api/imports', {
    headers: {
      Origin: 'https://attacker.example',
      'X-CSRF-Token': session.body.csrf_token,
      'Content-Type': 'application/json',
    },
    data: { name: 'hostile', config: 'hostname hostile' },
  });
  expect(response.status()).toBe(403);
});
