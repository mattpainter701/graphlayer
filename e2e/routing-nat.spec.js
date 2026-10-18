const { test, expect } = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');

const screenshotDir = process.env.GRAPHLAYER_ROUTING_NAT_SCREENSHOTS || path.resolve('test-results');

async function saveLayout(page, name, fullPage = true) {
  await fs.mkdir(screenshotDir, { recursive: true });
  await page.screenshot({ path: path.join(screenshotDir, name), fullPage });
}

async function importPathSample(page, mutate) {
  await page.goto('/#imports');
  const sample = await page.evaluate(async () => {
    const response = await fetch('/api/sample/path', { credentials: 'same-origin' });
    return response.json();
  });
  const source = mutate ? mutate(sample.config) : sample.config;
  await page.locator('#config-input').fill(source);
  await page.getByRole('button', { name: /import configuration/i }).click();
  await expect(page.locator('#path-controls')).toBeVisible({ timeout: 10_000 });
  const selectedId = await page.locator('#snapshot-select').inputValue();
  return page.evaluate(async (id) => (await fetch(`/api/snapshots/${encodeURIComponent(id)}`)).json(), selectedId)
    .then(response => response.snapshot);
}

async function evaluatePath(page, { ingress = 'outside', source = '198.51.100.42', destination = '198.51.100.15', protocol = 'tcp', sourcePort = 52000, destinationPort = 443 } = {}) {
  await page.locator('#path-ingress').selectOption(ingress);
  await page.locator('#path-source').fill(source);
  await page.locator('#path-destination').fill(destination);
  await page.locator('#path-protocol').selectOption(protocol);
  await page.locator('#path-source-port').fill(sourcePort ? String(sourcePort) : '');
  await page.locator('#path-destination-port').fill(destinationPort ? String(destinationPort) : '');
  await page.locator('#path-evaluate-button').click();
  await expect(page.locator('#path-result')).toBeVisible({ timeout: 10_000 });
  return page.evaluate(async () => (await fetch('/api/path-investigations')).json());
}

test('configuration import traces DNAT against real ACL tuple and restores exact saved path export', async ({ page }) => {
  const snapshot = await importPathSample(page);
  expect(snapshot.path.version).toBe('asa-path-v1');
  expect(snapshot.path.interfaces.every(item => item.evidence_refs.length > 0)).toBe(true);

  const history = await evaluatePath(page);
  await expect(page.locator('#path-disposition')).toContainText('MODELED FORWARD');
  await expect(page.locator('#path-route-summary')).toContainText('dmz');
  await expect(page.locator('#path-stages')).toContainText('10.20.4.15');
  await expect(page.locator('#path-stages')).toContainText(/lines \d+–\d+/i);
  await saveLayout(page, 'graphlayer-routing-nat.png');
  expect(history.investigations.filter(item => item.snapshot_id === snapshot.id)).toHaveLength(1);

  const blockedHistory = await evaluatePath(page, { destinationPort: 22 });
  await expect(page.locator('#path-disposition')).toContainText('BLOCKED');
  await expect(page.locator('#path-stages')).toContainText(/blocked/i);
  const snapshotHistory = blockedHistory.investigations.filter(item => item.snapshot_id === snapshot.id);
  expect(snapshotHistory).toHaveLength(2);

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.locator('#path-export-button').click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/^path-investigation-[a-f0-9]+\.json$/);
  const exported = JSON.parse(await fs.readFile(await download.path(), 'utf8'));
  const saved = await page.evaluate(async id => (await fetch(`/api/path-investigations/${encodeURIComponent(id)}`)).json(), exported.id);
  expect(exported).toEqual(saved.investigation);
  await page.reload();
  const snapshotRows = page.locator('#path-history-list .history-row').filter({ hasText: `snapshot ${snapshot.id}` });
  await expect(snapshotRows).toHaveCount(snapshotHistory.length);
  await snapshotRows.first().locator('.history-open').click();
  await expect(page.locator('#path-result')).toBeVisible();
  await expect(page.locator('#path-disposition')).toContainText(/MODELED FORWARD|BLOCKED/);
  await expect(page.locator('#path-result-scope')).toContainText(snapshot.hostname);
  expect(exported.id).toMatch(/^[a-f0-9]{64}$/);
});

test('outbound source NAT uses configured egress and a more specific route next hop', async ({ page }) => {
  const snapshot = await importPathSample(page, config => config.replace('nat (inside,outside) static 10.0.0.20', 'nat (inside,outside) static 198.51.100.20'));
  const summaries = await evaluatePath(page, { ingress: 'inside', source: '10.0.0.20', destination: '203.0.113.10' });
  await expect(page.locator('#path-disposition')).toContainText('MODELED FORWARD');
  await expect(page.locator('#path-route-summary')).toContainText('203.0.113.0/24');
  await expect(page.locator('#path-route-summary')).toContainText('192.0.2.253');
  await expect(page.locator('#path-stages')).toContainText('source_nat');
  await expect(page.locator('#path-stages')).toContainText('198.51.100.20');
  const saved = await page.evaluate(async id => (await fetch(`/api/path-investigations/${encodeURIComponent(id)}`)).json(), summaries.investigations[0].id);
  const routeRef = saved.investigation.result.selected_route.evidence_refs[0];
  const routeEvidence = snapshot.evidence.find(item => item.id === routeRef);
  expect(routeEvidence, 'selected specific route has physical evidence').toBeTruthy();
  const routeStage = page.locator('#path-stages .path-stage').filter({ has: page.locator('.path-stage-title strong', { hasText: /^route$/i }) });
  await routeStage.getByRole('button', { name: `${routeEvidence.kind} · lines ${routeEvidence.start_line}–${routeEvidence.end_line}` }).click();
  await expect(page.locator('#source-lines')).toBeVisible();
  await expect(page.locator('#source-text')).toContainText('route outside 203.0.113.0 255.255.255.0 192.0.2.253 5');
});

test('unsupported dynamic routing and manual NAT remain indeterminate', async ({ page }) => {
  const dynamic = await importPathSample(page, config => `${config}\nrouter ospf 1\n`);
  await evaluatePath(page, { destination: '10.20.4.15' });
  await expect(page.locator('#path-disposition')).toContainText('INDETERMINATE');
  expect(dynamic.path.routing_complete).toBe(false);
  const routingWarning = dynamic.path.warnings.find(warning => warning.code === 'dynamic_routing');
  expect(routingWarning, 'dynamic routing warning retains its source line').toBeTruthy();
  await expect(page.locator('#path-reasons')).toContainText(`dynamic_routing · physical line ${routingWarning.line}`);

  const twice = await importPathSample(page, config => `${config}\nnat (inside,outside) source static IDENTITY-CLIENT IDENTITY-CLIENT destination static WEB WEB\n`);
  await evaluatePath(page, { destination: '10.20.4.15' });
  await expect(page.locator('#path-disposition')).toContainText('INDETERMINATE');
  expect(twice.path.nat_complete).toBe(false);
  const natWarning = twice.path.warnings.find(warning => warning.code === 'unsupported_nat');
  expect(natWarning, 'unsupported NAT warning retains its source line').toBeTruthy();
  await expect(page.locator('#path-reasons')).toContainText(`unsupported_nat · physical line ${natWarning.line}`);
});

test('path UI ignores a delayed response after the flow tuple changes', async ({ page }) => {
  const original = await importPathSample(page);
  const sample = await page.evaluate(async () => (await fetch('/api/sample/path')).json());
  await page.locator('#config-input').fill(`${sample.config}\n! alternate path snapshot\n`);
  await page.getByRole('button', { name: /import configuration/i }).click();
  await expect.poll(() => page.locator('#snapshot-select').inputValue()).not.toBe(original.id);
  const alternateId = await page.locator('#snapshot-select').inputValue();
  await page.locator('#snapshot-select').selectOption(original.id);
  await expect.poll(() => page.locator('#snapshot-select').inputValue()).toBe(original.id);

  const deferred = () => {
    let resolve;
    const promise = new Promise(done => { resolve = done; });
    return { promise, resolve };
  };
  const started = [deferred(), deferred()];
  const gates = [deferred(), deferred()];
  let requestIndex = 0;
  await page.route('**/api/snapshots/*/path-evaluate', async route => {
    const response = await route.fetch();
    const body = await response.body();
    const index = requestIndex++;
    started[index].resolve();
    await gates[index].promise;
    await route.fulfill({ response, body });
  });
  await page.locator('#path-ingress').selectOption('outside');
  await page.locator('#path-evaluate-button').click();
  await started[0].promise;
  await expect(page.locator('#path-destination')).toBeDisabled();
  await page.locator('#path-destination').evaluate(node => {
    node.value = '198.51.100.99';
    node.dispatchEvent(new Event('input', { bubbles: true }));
  });
  gates[0].resolve();
  await expect(page.locator('#toast')).toContainText(/inputs changed/i);
  await expect(page.locator('#path-result')).toBeHidden();

  await expect(page.locator('#path-evaluate-button')).toBeEnabled();
  await page.locator('#path-destination').fill('198.51.100.15');
  await page.locator('#path-evaluate-button').click();
  await started[1].promise;
  await page.locator('#snapshot-select').evaluate((node, id) => {
    node.value = id;
    node.dispatchEvent(new Event('change', { bubbles: true }));
  }, alternateId);
  await expect.poll(() => page.locator('#snapshot-select').inputValue()).toBe(alternateId);
  await page.locator('#path-ingress').evaluate(node => {
    node.value = 'outside';
    node.dispatchEvent(new Event('change', { bubbles: true }));
  });
  gates[1].resolve();
  await expect(page.locator('#toast')).toContainText(/selected snapshot and ingress/i);
  await expect(page.locator('#path-result')).toBeHidden();
});

test('routing and NAT result remains readable on a narrow mobile viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await importPathSample(page);
  await evaluatePath(page);
  await expect(page.locator('#path-disposition')).toContainText('MODELED FORWARD');
  const widths = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  expect(widths.document, JSON.stringify(widths)).toBeLessThanOrEqual(widths.viewport);
  expect(widths.body, JSON.stringify(widths)).toBeLessThanOrEqual(widths.viewport);
  await saveLayout(page, 'graphlayer-routing-nat-mobile.png');
  await page.locator('#path-result').scrollIntoViewIfNeeded();
  await saveLayout(page, 'graphlayer-routing-nat-mobile-viewport.png', false);
});
