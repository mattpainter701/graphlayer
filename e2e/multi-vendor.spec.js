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

function changeConfig(sample, node, from, to) {
  const device = sample.configs.find((item) => item.node_id === node);
  expect(device, `sample includes ${node}`).toBeTruthy();
  expect(device.config, `mutation source exists in ${node}`).toContain(from);
  device.config = device.config.replace(from, to);
}

async function loadSample(page, mutate) {
  await page.goto('/');
  if (mutate) {
    await page.route('**/api/sample/network-multivendor', async (route) => {
      const response = await route.fetch();
      const sample = await response.json();
      mutate(sample);
      await route.fulfill({ response, json: sample });
    });
  }
  await page.locator('#network-load-vendor-sample').click();
  await expect(page.locator('#network-status')).toContainText(/3 sample firewall snapshots imported/i);
  await expect(page.locator('.network-node-row')).toHaveCount(3);
  await expect(page.locator('.network-link-row')).toHaveCount(2);
  expect(await page.locator('.network-node-id').evaluateAll((inputs) => inputs.map((input) => input.value))).toEqual(['edge', 'transit', 'core']);
  expect(await page.locator('.network-link-a-node').evaluateAll((inputs) => inputs.map((input) => input.value))).toEqual(['edge', 'transit']);
  expect(await page.locator('.network-link-b-node').evaluateAll((inputs) => inputs.map((input) => input.value))).toEqual(['transit', 'core']);
  await expect(page.locator('#network-source-node')).toHaveValue('edge');
  await expect(page.locator('#network-destination-node')).toHaveValue('core');
  await expect(page.locator('#network-flow-source')).toHaveValue('10.1.0.42');
  await expect(page.locator('#network-flow-destination')).toHaveValue('203.0.113.15');
  await expect(page.locator('#network-evaluate')).toBeEnabled();
}

async function evaluate(page) {
  await page.locator('#network-evaluate').click();
  await expect(page.locator('#network-result')).toBeVisible();
}

async function expectEvidenceReadableAndOwned(page, hops, expectedNodes) {
  for (let index = 0; index < expectedNodes.length; index++) {
    const hop = hops.nth(index);
    await expect(hop).toContainText(expectedNodes[index]);
    const evidenceButton = hop.locator('.evidence-link').first();
    await expect(evidenceButton, `${expectedNodes[index]} has a source evidence link`).toBeVisible();
    const ref = await evidenceButton.evaluate((button) => ({
      snapshotId: button.dataset.snapshotId,
      evidenceRef: button.dataset.evidenceRef,
    }));
    expect(ref.snapshotId).toMatch(/^[a-f0-9]{64}$/);
    expect(ref.evidenceRef).toBeTruthy();
    const detail = await page.evaluate(async (id) => {
      const response = await fetch(`/api/snapshots/${encodeURIComponent(id)}`);
      return response.json();
    }, ref.snapshotId);
    const snapshot = detail.snapshot;
    expect(snapshot.vendor).toBe(['paloalto-panos', 'cisco-asa', 'fortinet-fortigate'][index]);
    const evidence = snapshot.evidence.find((item) => item.id === ref.evidenceRef);
    expect(evidence, 'evidence reference belongs to the selected snapshot').toBeTruthy();
    const expectedLines = snapshot.source_lines
      .filter((line) => line.number >= evidence.start_line && line.number <= evidence.end_line)
      .map((line) => line.text)
      .filter((line) => line.trim());
    expect(expectedLines.length, 'evidence resolves to readable source lines').toBeGreaterThan(0);
    await evidenceButton.click();
    await expect(page.locator('#source-lines')).toBeVisible();
    await expect(page.locator('#source-caption')).toContainText(snapshot.hostname);
    for (const line of expectedLines) await expect(page.locator('#source-text')).toContainText(line);
    await page.locator('#close-source').click();
  }
}

test('traces three native vendors through translated forward and return tuples with evidence, export, and replay', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await loadSample(page);
  await evaluate(page);
  await expect(page.locator('#network-disposition')).toContainText(/modeled round trip/i, { timeout: 15_000 });

  const forward = page.locator('#network-forward-hops .path-stage');
  const returning = page.locator('#network-return-hops .path-stage');
  await expect(forward).toHaveCount(3);
  await expect(returning).toHaveCount(3);
  for (const [index, vendor] of ['Palo Alto PAN-OS', 'Cisco ASA', 'FortiGate FortiOS'].entries()) {
    await expect(forward.nth(index)).toContainText(vendor);
  }
  for (const [index, vendor] of ['FortiGate FortiOS', 'Cisco ASA', 'Palo Alto PAN-OS'].entries()) {
    await expect(returning.nth(index)).toContainText(vendor);
  }
  await expect(forward.nth(0)).toContainText('10.1.0.42');
  await expect(forward.nth(0)).toContainText('198.51.100.42');
  await expect(forward.nth(1)).toContainText('198.51.100.42');
  await expect(forward.nth(1)).toContainText('203.0.113.15');
  await expect(forward.nth(2)).toContainText('203.0.113.15');
  await expect(forward.nth(2)).toContainText('10.2.0.15');
  await expect(returning.nth(0)).toContainText('10.2.0.15');
  await expect(returning.nth(0)).toContainText('203.0.113.15');
  await expect(returning.nth(1)).toContainText('203.0.113.15');
  await expect(returning.nth(1)).toContainText('198.51.100.42');
  await expect(returning.nth(2)).toContainText('198.51.100.42');
  await expect(returning.nth(2)).toContainText('10.1.0.42');
  await expectEvidenceReadableAndOwned(page, forward, ['edge', 'transit', 'core']);
  await expect(page.locator('#network-assumptions')).toContainText(/established/i);

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.locator('#network-export').click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/^network-investigation-.*\.json$/i);
  const exported = JSON.parse(await readDownload(download));
  expect(exported.result.disposition).toBe('modeled_round_trip');
  expect(exported.result.forward.hops).toHaveLength(3);
  expect(exported.result.return.hops).toHaveLength(3);
  await expect(page.locator('#network-history-list .network-history-open').first()).toBeVisible();
  await page.locator('#network-history-list .network-history-open').first().click();
  await expect(page.locator('#network-disposition')).toContainText(/modeled round trip/i);
  await expect(page.locator('#network-forward-hops .path-stage')).toHaveCount(3);

  await fs.mkdir(screenshotRoot, { recursive: true });
  await page.screenshot({ path: path.join(screenshotRoot, 'multi-vendor-desktop.png'), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: path.join(screenshotRoot, 'multi-vendor-mobile.png'), fullPage: true });
  const dimensions = await page.evaluate(() => ({ viewport: window.innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  expect(dimensions.viewport).toBe(390);
  expect(dimensions.document, JSON.stringify(dimensions)).toBeLessThanOrEqual(390);
  expect(dimensions.body, JSON.stringify(dimensions)).toBeLessThanOrEqual(390);
});

test('loads each vendor sample through its native format selector', async ({ page }) => {
  await page.goto('/');
  await page.locator('#load-panos-sample').click();
  await expect(page.locator('#config-vendor')).toHaveValue('paloalto-panos');
  await expect(page.locator('#config-input')).toHaveValue(/set /);
  await page.locator('#load-fortigate-sample').click();
  await expect(page.locator('#config-vendor')).toHaveValue('fortinet-fortigate');
  await expect(page.locator('#config-input')).toHaveValue(/config system interface/);
});

test('applies PAN security policy to the original DNAT address and post-NAT zone', async ({ page }) => {
  await page.goto('/');
  await page.locator('#load-panos-sample').click();
  const configInput = page.locator('#config-input');
  const original = await configInput.inputValue();
  expect(original).toContain('set rulebase nat rules client-static source-translation static-ip translated-address 198.51.100.42');
  expect(original).toContain('set rulebase security rules allow-https to untrust');
  const dnat = original
    .replace('set rulebase nat rules client-static source-translation static-ip translated-address 198.51.100.42', 'set rulebase nat rules client-static destination-translation translated-address 10.3.0.50')
    .replace('set rulebase nat rules client-static destination any', 'set rulebase nat rules client-static destination backend-203-0-113-15')
    .replace('set network interface ethernet ethernet1/2 layer3 ip 172.16.0.1/30', 'set network interface ethernet ethernet1/2 layer3 ip 172.16.0.1/30\nset network interface ethernet ethernet1/3 layer3 ip 10.3.0.1/24')
    .replace('set zone untrust network layer3 ethernet1/2', 'set zone untrust network layer3 ethernet1/2\nset zone dmz network layer3 ethernet1/3')
    .replace('set network virtual-router default interface ethernet1/2', 'set network virtual-router default interface ethernet1/2\nset network virtual-router default interface ethernet1/3')
    .replace('set rulebase security rules allow-https to untrust', 'set rulebase security rules allow-https to dmz');
  await configInput.fill(dnat);
  await page.locator('#import-button').click();
  await expect(page.locator('#path-controls')).toBeVisible();
  await page.locator('#path-ingress').selectOption('ethernet1/1');
  await page.locator('#path-source').fill('10.1.0.42');
  await page.locator('#path-destination').fill('203.0.113.15');
  await page.locator('#path-protocol').selectOption('tcp');
  await page.locator('#path-source-port').fill('52000');
  await page.locator('#path-destination-port').fill('443');
  await page.locator('#path-evaluate-button').click();
  await expect(page.locator('#path-result')).toBeVisible();
  await expect(page.locator('#path-disposition')).toContainText(/modeled forward/i);
  const tuples = page.locator('#path-flow-summary .path-tuple');
  await expect(tuples).toHaveCount(3);
  await expect(tuples.nth(1)).toContainText('203.0.113.15');
  await expect(tuples.nth(2)).toContainText('10.3.0.50');
  const dnatStage = page.locator('#path-stages .path-stage').filter({ has: page.locator('.path-stage-title strong', { hasText: /^destination_nat$/i }) });
  const policyStage = page.locator('#path-stages .path-stage').filter({ has: page.locator('.path-stage-title strong', { hasText: /^ingress_policy$/i }) });
  await expect(dnatStage).toContainText('10.3.0.50');
  await expect(policyStage).toContainText(/original packet tuple/i);
});

test('keeps unsupported PAN App-ID semantics indeterminate', async ({ page }) => {
  await loadSample(page, (sample) => changeConfig(
    sample,
    'edge',
    'set rulebase security rules allow-https application any',
    'set rulebase security rules allow-https application web-browsing',
  ));
  await evaluate(page);
  await expect(page.locator('#network-disposition')).toContainText(/indeterminate/i);
  await expect(page.locator('#network-reasons')).toContainText(/vendor routing, policy, or translation evidence is incomplete/i);
  await expect(page.locator('#network-forward-hops .path-stage').first()).toContainText(/application|app.?id/i);
});

test('keeps unsupported FortiGate central NAT indeterminate', async ({ page }) => {
  await loadSample(page, (sample) => {
    const core = sample.configs.find((item) => item.node_id === 'core');
    core.config += 'config firewall central-snat-map\n edit 1\n set srcintf "port2"\n set dstintf "port1"\n next\nend\n';
  });
  await evaluate(page);
  await expect(page.locator('#network-disposition')).toContainText(/indeterminate/i);
  await expect(page.locator('#network-reasons')).toContainText(/vendor routing, policy, or translation evidence is incomplete/i);
  await expect(page.locator('#network-forward-hops .path-stage').last()).toContainText(/central.?nat/i);
});

test('preserves FortiGate CLI policy order when numeric identifiers run backwards', async ({ page }) => {
  await loadSample(page, (sample) => {
    const core = sample.configs.find((item) => item.node_id === 'core');
    const before = core.config;
    core.config = core.config
      .replace('edit 10\n', 'edit __TEMP__\n')
      .replace('edit 20\n', 'edit 10\n')
      .replace('edit __TEMP__\n', 'edit 20\n')
      .replace('set srcintf "port2"', 'set srcintf "port1"')
      .replace('set dstintf "port1"', 'set dstintf "port2"');
    expect(core.config).not.toBe(before);
    expect(core.config.indexOf('edit 20')).toBeLessThan(core.config.indexOf('edit 10'));
  });
  await evaluate(page);
  await expect(page.locator('#network-disposition')).toContainText(/modeled round trip/i);
  await expect(page.locator('#network-forward-hops .path-stage')).toHaveCount(3);
  await expect(page.locator('#network-forward-hops .path-stage').nth(2)).toContainText('FortiGate FortiOS');
});

for (const scenario of [
  {
    name: 'FortiGate VIP reference mismatch',
    mutate: (sample) => changeConfig(sample, 'core', 'set dstaddr "WEB"', 'set dstaddr "MISSING-WEB"'),
    disposition: /indeterminate/i,
  },
  {
    name: 'missing FortiGate return route',
    mutate: (sample) => changeConfig(sample, 'core', 'set dst 198.51.100.42 255.255.255.255', 'set dst 198.51.100.41 255.255.255.255'),
    disposition: /blocked/i,
  },
  {
    name: 'FortiGate policy block',
    mutate: (sample) => changeConfig(sample, 'core', 'set action accept', 'set action deny'),
    disposition: /blocked/i,
  },
]) {
  test(`reports ${scenario.name} using imported configuration`, async ({ page }) => {
    await loadSample(page, scenario.mutate);
    await evaluate(page);
    await expect(page.locator('#network-disposition')).toContainText(scenario.disposition);
    await expect(page.locator('#network-forward-hops .path-stage')).toHaveCount(3);
  });
}

test('discards a late result if vendor topology changes during evaluation', async ({ page }) => {
  await loadSample(page);
  const priorHistoryCount = await page.locator('#network-history-list .network-history-open').count();
  let release;
  let signal;
  const arrived = new Promise((resolve) => { signal = resolve; });
  const released = new Promise((resolve) => { release = resolve; });
  await page.route('**/api/network-investigations', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    const response = await route.fetch();
    signal();
    await released;
    await route.fulfill({ response });
  });
  await page.locator('#network-evaluate').click();
  await arrived;
  const nodeId = page.locator('.network-node-id').first();
  await nodeId.evaluate((input) => {
    input.disabled = false;
    input.value = 'changed-edge';
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
  release();
  await expect(page.locator('#toast')).toContainText(/topology or flow changed while the request was running/i);
  await expect(page.locator('#network-result')).toBeHidden();
  await expect(page.locator('#network-history-list .network-history-open')).toHaveCount(priorHistoryCount);
});
