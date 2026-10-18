const { test, expect } = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');

test.use({ baseURL: 'http://127.0.0.1:8089' });

const screenshotRoot = process.env.GRAPHLAYER_E2E_SCREENSHOT_DIR || path.resolve('test-results');

async function readDownload(download) {
  const stream = await download.createReadStream();
  const chunks = [];
  for await (const chunk of stream) chunks.push(chunk);
  return Buffer.concat(chunks).toString('utf8');
}

async function loadSample(page, mutate) {
  await page.goto('/');
  if (mutate) {
    await page.route('**/api/sample/network-exploration', async (route) => {
      const response = await route.fetch();
      const sample = await response.json();
      mutate(sample);
      await route.fulfill({ response, json: sample });
    });
  }
  await page.locator('#network-load-exploration-sample').click();
  await expect(page.locator('#network-status')).toContainText(/4 sample firewall snapshots imported/i);
  await expect(page.locator('.network-node-row')).toHaveCount(4);
  await expect(page.locator('.network-link-row')).toHaveCount(4);
  expect(await page.locator('.network-node-id').evaluateAll((inputs) => inputs.map((input) => input.value))).toEqual(['edge', 'north', 'south', 'core']);
  await expect(page.locator('#network-source-node')).toHaveValue('edge');
  await expect(page.locator('#network-destination-node')).toHaveValue('core');
  await expect(page.locator('#network-flow-source')).toHaveValue('10.1.0.42');
  await expect(page.locator('#network-flow-destination')).toHaveValue('203.0.113.15');
  await expect(page.locator('#network-explore')).toBeEnabled();
}

async function explore(page) {
  let responseBody;
  await page.route('**/api/network-explorations', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    const response = await route.fetch();
    responseBody = await response.json();
    await route.fulfill({ response });
  });
  await page.locator('#network-explore').click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  expect(responseBody && responseBody.exploration && responseBody.exploration.result).toBeTruthy();
  return responseBody.exploration;
}

async function expectEvidenceReadableAndOwned(page, hopLocator) {
  const hops = hopLocator.locator('.path-stage');
  const count = await hops.count();
  expect(count).toBeGreaterThan(0);
  for (let index = 0; index < count; index++) {
    const hop = hops.nth(index);
    const owner = await hop.getAttribute('data-node-id');
    await expect(hop).toContainText(owner);
    const evidenceButton = hop.locator('.evidence-link').first();
    await expect(evidenceButton).toBeVisible();
    const ref = await evidenceButton.evaluate((button) => ({ snapshotId: button.dataset.snapshotId, evidenceRef: button.dataset.evidenceRef }));
    expect(ref.snapshotId).toMatch(/^[a-f0-9]{64}$/);
    expect(ref.evidenceRef).toBeTruthy();
    const detail = await page.evaluate(async (id) => (await fetch(`/api/snapshots/${encodeURIComponent(id)}`)).json(), ref.snapshotId);
    const snapshot = detail.snapshot;
    expect(snapshot.hostname.toLowerCase()).toContain(owner);
    const evidence = snapshot.evidence.find((item) => item.id === ref.evidenceRef);
    expect(evidence).toBeTruthy();
    const lines = snapshot.source_lines.filter((line) => line.number >= evidence.start_line && line.number <= evidence.end_line).map((line) => line.text).filter((line) => line.trim());
    expect(lines.length).toBeGreaterThan(0);
    await evidenceButton.click();
    await expect(page.locator('#source-lines')).toBeVisible();
    await expect(page.locator('#source-caption')).toContainText(snapshot.hostname);
    for (const line of lines) await expect(page.locator('#source-text')).toContainText(line);
    await page.locator('#close-source').click();
  }
}

test('explores both firewall branches, asymmetric and symmetric returns, source evidence, export, and exact replay', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await loadSample(page);
  const exploration = await explore(page);
  const result = exploration.result;
  expect(result.branches).toHaveLength(2);
  expect(result.complete).toBe(true);
  expect(result.truncated).toBe(false);
  expect(result.disposition).toBe('indeterminate');
  const north = result.branches.find((branch) => branch.forward.hops.some((hop) => hop.node_id === 'north'));
  const south = result.branches.find((branch) => branch.forward.hops.some((hop) => hop.node_id === 'south'));
  expect(north).toBeTruthy();
  expect(south).toBeTruthy();
  expect(north.returns.length).toBeGreaterThan(0);
  expect(south.returns.length).toBeGreaterThan(0);
  const northReturn = north.returns.find((candidate) => candidate.asymmetric);
  const southReturn = south.returns.find((candidate) => !candidate.asymmetric);
  expect(northReturn).toBeTruthy();
  expect(southReturn).toBeTruthy();

  await expect(page.locator('#exploration-disposition')).toContainText(/indeterminate/i);
  await expect(page.locator('#exploration-coverage')).toContainText(/complete/i);
  await expect(page.locator('#exploration-coverage')).toContainText(/not truncated/i);
  const branchSelect = page.locator('#exploration-branch');
  const northIndex = result.branches.indexOf(north);
  await branchSelect.selectOption(String(northIndex));
  await page.locator('#exploration-return').selectOption(String(north.returns.indexOf(northReturn)));
  await expect(page.locator('#exploration-return-status')).toContainText(/asymmetric route candidate/i);
  await expect(page.locator('#exploration-forward-hops')).toContainText('north');
  await expect(page.locator('#exploration-return-hops')).toContainText('south');
  await expect(page.locator('#exploration-forward-hops .path-stage').first()).toContainText('10.1.0.42');
  await expect(page.locator('#exploration-forward-hops')).toContainText('198.51.100.42');
  await expect(page.locator('#exploration-forward-hops')).toContainText('10.2.0.15');
  await expectEvidenceReadableAndOwned(page, page.locator('#exploration-forward-hops'));
  await expectEvidenceReadableAndOwned(page, page.locator('#exploration-return-hops'));

  await page.locator('#exploration-topology-list button.topology-list-node').filter({ hasText: 'south' }).click();
  await expect(page.locator('#exploration-return-hops .path-stage[data-node-id="south"]')).toBeFocused();
  await branchSelect.selectOption(String(result.branches.indexOf(south)));
  await page.locator('#exploration-return').selectOption(String(south.returns.indexOf(southReturn)));
  await expect(page.locator('#exploration-return-status')).toContainText(/session established/i);
  await expect(page.locator('#exploration-forward-hops')).toContainText('south');
  await expect(page.locator('#exploration-return-hops')).toContainText('south');
  await expect(page.locator('#exploration-topology g.topology-node[data-node-id="north"]')).toHaveAttribute('aria-label', /not in selected branch/i);

  const [download] = await Promise.all([page.waitForEvent('download'), page.locator('#exploration-export').click()]);
  expect(download.suggestedFilename()).toMatch(/^network-exploration-.*\.json$/i);
  const exported = JSON.parse(await readDownload(download));
  expect(exported.request).toEqual(exploration.request);
  expect(exported.result.branches.map((branch) => branch.id)).toEqual(result.branches.map((branch) => branch.id));
  await expect(page.locator('#exploration-history-list .exploration-history-open').first()).toBeVisible();
  await page.locator('#exploration-history-list .exploration-history-open').first().click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  await expect(page.locator('#exploration-branch option')).toHaveCount(2);
  expect(await page.locator('#network-source-node').inputValue()).toBe(exploration.request.source.node_id);
  expect(await page.locator('#network-flow-source').inputValue()).toBe(exploration.request.flow.source);

  await fs.mkdir(screenshotRoot, { recursive: true });
  await page.screenshot({ path: path.join(screenshotRoot, 'path-exploration-desktop.png'), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: path.join(screenshotRoot, 'path-exploration-mobile.png'), fullPage: true });
  const dimensions = await page.evaluate(() => ({ viewport: window.innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  expect(dimensions.viewport).toBe(390);
  expect(dimensions.document, JSON.stringify(dimensions)).toBeLessThanOrEqual(390);
  expect(dimensions.body, JSON.stringify(dimensions)).toBeLessThanOrEqual(390);
});

test('shows a blocked candidate within the mixed branch result', async ({ page }) => {
  await loadSample(page, (sample) => {
    const north = sample.configs.find((config) => config.node_id === 'north');
    north.config = north.config.replace('access-list LEFT-IN extended permit tcp host 198.51.100.42 host 203.0.113.15 eq 443', 'access-list LEFT-IN extended deny tcp host 198.51.100.42 host 203.0.113.15 eq 443');
  });
  const exploration = await explore(page);
  const blocked = exploration.result.branches.find((branch) => branch.disposition === 'blocked');
  expect(blocked).toBeTruthy();
  const index = exploration.result.branches.indexOf(blocked);
  await page.locator('#exploration-branch').selectOption(String(index));
  await expect(page.locator('#exploration-forward-hops')).toContainText('north');
  await expect(page.locator('#exploration-forward-hops')).toContainText(/blocked/i);
});

test('reports unresolved return coverage when the core has no route to the translated source', async ({ page }) => {
  await loadSample(page, (sample) => {
    const core = sample.configs.find((config) => config.node_id === 'core');
    expect(core.config).toContain('set dst 198.51.100.42 255.255.255.255');
    core.config = core.config.replace('set dst 198.51.100.42 255.255.255.255', 'set dst 198.51.100.41 255.255.255.255');
  });
  const exploration = await explore(page);
  expect(exploration.result.branches).toHaveLength(2);
  expect(exploration.result.disposition).toMatch(/indeterminate|blocked/);
  for (const branch of exploration.result.branches) {
    expect(branch.returns.length).toBeGreaterThan(0);
    expect(branch.returns.some((candidate) => candidate.direction.disposition === 'blocked' || candidate.direction.disposition === 'indeterminate')).toBe(true);
  }
});

test('discards a late exploration response after the topology changes', async ({ page }) => {
  await loadSample(page);
  let release;
  let signal;
  const arrived = new Promise((resolve) => { signal = resolve; });
  const released = new Promise((resolve) => { release = resolve; });
  await page.route('**/api/network-explorations', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    const response = await route.fetch();
    signal();
    await released;
    await route.fulfill({ response });
  });
  await page.locator('#network-explore').click();
  await arrived;
  const nodeId = page.locator('.network-node-id').first();
  await nodeId.evaluate((input) => {
    input.disabled = false;
    input.value = 'changed-edge';
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
  release();
  await expect(page.locator('#toast')).toContainText(/topology or flow changed while the request was running/i);
  await expect(page.locator('#exploration-result')).toBeHidden();
});
