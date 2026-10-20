const { test, expect } = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');

const screenshotRoot = process.env.GRAPHLAYER_E2E_SCREENSHOT_DIR || path.resolve('test-results');

async function readDownload(download) {
  const stream = await download.createReadStream();
  const chunks = [];
  for await (const chunk of stream) chunks.push(chunk);
  return Buffer.concat(chunks).toString('utf8');
}

async function loadNetworkSample(page, mutateSample) {
  await page.goto('/#network-path');
  if (mutateSample) {
    await page.route('**/api/sample/network', async (route) => {
      const response = await route.fetch();
      const sample = await response.json();
      mutateSample(sample);
      await route.fulfill({ response, json: sample });
    });
  }
  await page.getByRole('button', { name: /load multi-firewall sample/i }).click();
  await expect(page.locator('#network-status')).toContainText(/2 sample firewall snapshots imported/i);
  await expect(page.locator('.network-node-row')).toHaveCount(2);
  await expect(page.locator('.network-link-row')).toHaveCount(1);
  await expect(page.locator('.network-link-a-node')).toHaveValue('edge');
  await expect(page.locator('.network-link-b-node')).toHaveValue('core');
  await expect(page.locator('#network-evaluate')).toBeEnabled();
}

async function evaluateNetwork(page) {
  await page.getByRole('button', { name: /trace network path/i }).click();
  await expect(page.locator('#network-result')).toBeVisible();
}

function replaceCoreConfig(sample, from, to) {
  const core = sample.configs.find((item) => item.node_id === 'core');
  core.config = core.config.replace(from, to);
}

test('models the two-firewall NAT round trip, opens hop evidence, exports, and restores history', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await loadNetworkSample(page);
  await evaluateNetwork(page);
  await expect(page.locator('#network-disposition')).toContainText(/modeled round trip/i, { timeout: 15_000 });
  await expect(page.locator('#topology-caption')).toContainText(/configured forward\/return trace/i);
  await expect(page.locator('#exploration-topology .topology-edge.topology-forward')).toHaveCount(1);
  await expect(page.locator('#exploration-topology .topology-edge.topology-return')).toHaveCount(1);
  await expect(page.locator('#exploration-topology .topology-node.topology-forward')).toHaveCount(2);
  await expect(page.locator('#exploration-topology .topology-node.topology-return')).toHaveCount(2);
  const forwardHops = page.locator('#network-forward-hops .path-stage');
  await expect(forwardHops).toHaveCount(2);
  await expect(forwardHops.nth(0)).toContainText('edge');
  await expect(forwardHops.nth(1)).toContainText('core');
  await expect(page.locator('#network-return-hops .path-stage')).toHaveCount(2);
  await expect(page.locator('#network-assumptions')).toContainText(/established/i);

  const nodes = await page.locator('.network-node-row').evaluateAll((rows) => rows.map((row) => ({
    id: row.querySelector('.network-node-id').value,
    snapshotId: row.querySelector('.network-snapshot-id').value,
  })));

  const coreSnapshotId = nodes.find((item) => item.id === 'core').snapshotId;
  const coreSnapshotResponse = await page.evaluate(async (id) => (await fetch(`/api/snapshots/${encodeURIComponent(id)}`)).json(), coreSnapshotId);
  const coreSnapshot = coreSnapshotResponse.snapshot;
  expect(coreSnapshot).toBeTruthy();
  await page.locator('#topology-device-list .topology-device[data-node-id="core"]').click();
  for (const group of ['topology', 'configuration', 'path', 'evidence']) {
    const section = page.locator(`#topology-inspector details[data-group="${group}"]`);
    await expect(section).toBeVisible();
    await section.locator('summary').click();
    await expect(section).toHaveAttribute('open', '');
  }
  await expect(page.locator('#topology-inspector')).toContainText(coreSnapshot.hostname);
  await expect(page.locator('#topology-inspector')).toContainText(coreSnapshotId);
  await expect(page.locator('#topology-inspector')).toContainText(/selected path context/i);
  await expect(page.locator('#topology-inspector')).toContainText('Saved configuration model');
  await expect(page.locator('#topology-inspector')).toContainText(/Forward trace/i);
  await expect(page.locator('#topology-inspector')).toContainText(/return trace/i);
  await expect(page.locator('#topology-inspector')).toContainText(/Forward/);
  await expect(page.locator('#topology-inspector')).toContainText(/Return/);
  const coreInterface = coreSnapshot.path.interfaces[0];
  const coreRoute = coreSnapshot.path.routes[0];
  expect(coreSnapshot.path.interfaces.length).toBeGreaterThan(0);
  expect(coreSnapshot.path.routes.length).toBeGreaterThan(0);
  await expect(page.locator('#topology-inspector')).toContainText(coreInterface.name || coreInterface.physical);
  await expect(page.locator('#topology-inspector')).toContainText(coreRoute.prefix);
  const inspectorEvidence = page.locator('#topology-inspector .topology-source-button').first();
  await expect(inspectorEvidence).toBeVisible();
  await inspectorEvidence.click();
  await expect(page.locator('#source-lines')).toBeVisible();
  await expect(page.locator('#source-caption')).toContainText(coreSnapshot.hostname);
  await expect(page.locator('#source-caption')).toContainText(coreSnapshotId.slice(0, 12));
  await page.getByRole('button', { name: /close source evidence/i }).click();

  for (const hopIndex of [0, 1]) {
    const hop = forwardHops.nth(hopIndex);
    const evidenceButton = hop.locator('.evidence-link').first();
    await expect(evidenceButton).toBeVisible();
    const info = await evidenceButton.evaluate((button) => ({
      snapshotId: button.dataset.snapshotId,
      ref: button.dataset.evidenceRef,
    }));
    const owner = nodes.find((node) => node.snapshotId === info.snapshotId);
    expect(owner, `evidence must resolve to the snapshot assigned to hop ${hopIndex + 1}`).toBeTruthy();
    expect(owner.id).toBe(hopIndex === 0 ? 'edge' : 'core');
    const detail = await page.evaluate(async (id) => {
      const response = await fetch(`/api/snapshots/${encodeURIComponent(id)}`);
      return response.json();
    }, info.snapshotId);
    const snapshot = detail.snapshot;
    const evidence = snapshot.evidence.find((item) => item.id === info.ref);
    expect(evidence, 'the evidence ID resolves within its owning snapshot').toBeTruthy();
    const expectedSource = snapshot.source_lines
      .filter((line) => line.number >= evidence.start_line && line.number <= evidence.end_line)
      .map((line) => line.text).filter((text) => text.trim());
    await evidenceButton.click();
    await expect(page.locator('#source-lines')).toBeVisible();
    await expect(page.locator('#source-caption')).toContainText(snapshot.hostname);
    for (const line of expectedSource) await expect(page.locator('#source-text')).toContainText(line);
    await page.getByRole('button', { name: /close source evidence/i }).click();
  }

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.locator('#network-export').click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/^network-investigation-.*\.json$/i);
  const exported = JSON.parse(await readDownload(download));
  expect(exported.result.disposition).toBe('modeled_round_trip');
  expect(exported.result.forward.hops).toHaveLength(2);
  expect(exported.result.return.hops).toHaveLength(2);

  await expect(page.locator('#network-history-list .network-history-open').first()).toBeVisible();
  await page.locator('#network-history-list .network-history-open').first().click();
  await expect(page.locator('#network-result')).toBeVisible();
  await expect(page.locator('#network-disposition')).toContainText(/modeled round trip/i);

  await fs.mkdir(screenshotRoot, { recursive: true });
  await page.screenshot({ path: path.join(screenshotRoot, 'multi-firewall-desktop.png'), fullPage: true });
  await page.locator('#network-result').evaluate((node) => node.scrollIntoView({ block: 'start' }));
  await page.screenshot({ path: path.join(screenshotRoot, 'multi-firewall-desktop-viewport.png') });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: path.join(screenshotRoot, 'multi-firewall-mobile.png'), fullPage: true });
  await page.locator('#network-result').evaluate((node) => node.scrollIntoView({ block: 'start' }));
  await page.screenshot({ path: path.join(screenshotRoot, 'multi-firewall-mobile-viewport.png') });
  const dimensions = await page.evaluate(() => ({ viewport: window.innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  expect(dimensions.viewport).toBe(390);
  expect(dimensions.document, JSON.stringify(dimensions)).toBeLessThanOrEqual(390);
  expect(dimensions.body, JSON.stringify(dimensions)).toBeLessThanOrEqual(390);
});

test('blocks when the second firewall denies the translated flow', async ({ page }) => {
  await loadNetworkSample(page, (sample) => replaceCoreConfig(
    sample,
    'access-list TRANSIT-IN extended permit tcp host 198.51.100.42 host 10.2.0.15 eq 443',
    'access-list TRANSIT-IN extended deny tcp host 198.51.100.42 host 10.2.0.15 eq 443',
  ));
  await evaluateNetwork(page);
  await expect(page.locator('#network-disposition')).toContainText(/blocked/i);
  const hops = page.locator('#network-forward-hops .path-stage');
  await expect(hops).toHaveCount(2);
  await expect(hops.nth(1)).toContainText(/blocked/i);
  await expect(hops.nth(1)).toContainText('core');
});

test('reports a complete forward path blocked by a missing configured return route', async ({ page }) => {
  await loadNetworkSample(page, (sample) => replaceCoreConfig(
    sample,
    'route transit 198.51.100.42 255.255.255.255 172.16.0.1\n',
    '',
  ));
  await evaluateNetwork(page);
  await expect(page.locator('#network-disposition')).toContainText(/blocked/i);
  await expect(page.locator('#network-forward-hops .path-stage')).toHaveCount(2);
  await expect(page.locator('#network-return-hops .path-stage')).toHaveCount(1);
  await expect(page.locator('#network-reasons')).toContainText(/no supported connected or static route/i);
});

test('keeps an asymmetric return next hop indeterminate after forward success', async ({ page }) => {
  await loadNetworkSample(page, (sample) => {
    sample.configs.forEach((config) => { config.config = config.config.replaceAll('255.255.255.252', '255.255.255.248'); });
    replaceCoreConfig(sample, 'route transit 198.51.100.42 255.255.255.255 172.16.0.1', 'route transit 198.51.100.42 255.255.255.255 172.16.0.3');
  });
  await evaluateNetwork(page);
  await expect(page.locator('#network-disposition')).toContainText(/indeterminate/i);
  await expect(page.locator('#network-forward-hops .path-stage')).toHaveCount(2);
  await expect(page.locator('#network-return-hops')).toContainText(/next hop|peer/i);
});

test('keeps unsupported twice NAT indeterminate', async ({ page }) => {
  await loadNetworkSample(page, (sample) => {
    const edge = sample.configs.find((item) => item.node_id === 'edge');
    edge.config += 'nat (inside,transit) source static CLIENT CLIENT destination static WEB WEB\n';
  });
  await evaluateNetwork(page);
  await expect(page.locator('#network-disposition')).toContainText(/indeterminate/i);
  await expect(page.locator('#network-reasons')).toContainText(/unsupported|NAT/i);
});

for (const mutation of [
  { name: 'flow tuple', selector: '#network-flow-source', value: '10.1.0.43' },
  { name: 'topology link', selector: '.network-link-a-interface', value: 'wrong-link' },
]) {
  test(`discards a late network response after the ${mutation.name} changes`, async ({ page }) => {
    await loadNetworkSample(page);
    const priorHistoryCount = await page.locator('#network-history-list .network-history-open').count();
    let releaseResponse;
    let signalRequest;
    const requestArrived = new Promise((resolve) => { signalRequest = resolve; });
    const responseReleased = new Promise((resolve) => { releaseResponse = resolve; });
    await page.route('**/api/network-investigations', async (route) => {
      if (route.request().method() !== 'POST') {
        await route.continue();
        return;
      }
      const response = await route.fetch();
      signalRequest();
      await responseReleased;
      await route.fulfill({ response });
    });
    await page.getByRole('button', { name: /trace network path/i }).click();
    await requestArrived;
    const changed = page.locator(mutation.selector);
    await expect(changed).toBeDisabled();
    await changed.evaluate((input, value) => {
      input.disabled = false;
      input.value = value;
      input.dispatchEvent(new Event(input.tagName === 'SELECT' ? 'change' : 'input', { bubbles: true }));
    }, mutation.value);
    releaseResponse();
    await expect(page.locator('#toast')).toContainText(/topology or flow changed while the request was running/i);
    await expect(page.locator('#network-result')).toBeHidden();
    await expect(page.locator('#network-history-list .network-history-open')).toHaveCount(priorHistoryCount);
  });
}
