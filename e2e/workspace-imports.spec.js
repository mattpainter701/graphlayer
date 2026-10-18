const { test, expect } = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');

test.use({ baseURL: 'http://127.0.0.1:8090' });
const screenshots = process.env.GRAPHLAYER_E2E_SCREENSHOT_DIR || path.resolve('test-results');

async function workspace(page) {
  const response = await page.request.get('/api/workspace');
  expect(response.ok()).toBe(true);
  return response.json();
}

async function sample(page) {
  const response = await page.request.get('/api/sample/network-exploration');
  expect(response.ok()).toBe(true);
  return response.json();
}

async function enterWorkspace(page) {
  await page.goto('/');
  await expect(page.locator('#workspace-title')).not.toHaveText(/Loading workspace/i);
  await expect(page.locator('#workspace-refresh')).toBeEnabled();
}

async function validate(page, count) {
  await page.locator('#batch-preview').click();
  await expect(page.locator('#batch-preview-results .preview-item')).toHaveCount(count);
}

async function publish(page, name) {
  await page.locator('#batch-set-name').fill(name);
  const responsePromise = page.waitForResponse((response) => response.request().method() === 'POST' && /\/api\/import-previews\/[^/]+\/publish$/.test(response.url()));
  await page.locator('#batch-publish').click();
  const response = await responsePromise;
  expect(response.ok()).toBe(true);
  const result = await response.json();
  await expect(page.locator('#active-set-name')).toHaveText(name);
  await expect(page.locator('#batch-publish')).toBeDisabled();
  return result.snapshot_set;
}

async function readDownload(download) {
  const stream = await download.createReadStream();
  const chunks = [];
  for await (const chunk of stream) chunks.push(chunk);
  return JSON.parse(Buffer.concat(chunks).toString('utf8'));
}

test('onboards a site and mixed-vendor baseline, investigates pinned members, and reopens workspace and evidence', async ({ page }, info) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await enterWorkspace(page);
  await page.locator('#workspace-name-input').fill('Enterprise network operations');
  await page.locator('#workspace-rename').click();
  await expect(page.locator('#workspace-title')).toHaveText('Enterprise network operations');

  const beforeSite = await workspace(page);
  if (!beforeSite.workspace.sites.some((site) => site.name === 'Chicago DC')) {
    await page.locator('#site-name').fill('Chicago DC');
    await page.locator('#site-description').fill('Firewall operations baseline');
    await page.locator('#site-create').click();
  }
  await expect(page.locator('#workspace-sites')).toContainText('Chicago DC');
  const site = (await workspace(page)).workspace.sites.find((item) => item.name === 'Chicago DC');
  const before = await page.request.get('/api/snapshots').then((response) => response.json());

  await page.locator('#batch-load-sample').click();
  await expect(page.locator('#batch-items .batch-item')).toHaveCount(4);
  await validate(page, 4);
  await expect(page.locator('#batch-preview-results .preview-item[data-status="ready"]')).toHaveCount(4);
  await expect(page.locator('#batch-preview-results')).toContainText('Palo Alto');
  await expect(page.locator('#batch-preview-results')).toContainText('Cisco ASA');
  await expect(page.locator('#batch-preview-results')).toContainText('FortiGate');
  await expect(page.locator('#batch-preview-results')).toContainText('branch-edge');
  const unpublished = await page.request.get('/api/snapshots').then((response) => response.json());
  expect(unpublished.snapshots.map((item) => item.id)).toEqual(before.snapshots.map((item) => item.id));
  await fs.mkdir(screenshots, { recursive: true });
  await page.locator('#workspace-home').screenshot({ path: path.join(screenshots, 'workspace-imports-preview.png') });
  await page.locator('#batch-site-select').selectOption(site.id);
  const setName = `Branch baseline ${info.retry}`;
  const set = await publish(page, setName);
  expect(set.members).toHaveLength(4);
  expect(set.site_id).toBe(site.id);
  await expect(page.locator('#active-set-site')).toContainText('Chicago DC');
  await expect(page.locator('#active-set-members .dataset-member')).toHaveCount(4);

  await fs.mkdir(screenshots, { recursive: true });
  await page.locator('#workspace-home').screenshot({ path: path.join(screenshots, 'workspace-imports-desktop.png') });
  await page.locator('#active-set-open-network').click();
  await expect(page.locator('.network-node-row')).toHaveCount(4);
  expect(await page.locator('.network-snapshot-id').evaluateAll((inputs) => inputs.map((input) => input.value))).toEqual(set.members.map((member) => member.snapshot_id));
  await expect(page.locator('.network-link-row')).toHaveCount(0);
  const ids = await page.locator('.network-node-id').evaluateAll((inputs) => inputs.map((input) => input.value));
  const links = [[0,'ethernet1/2',1,'left'],[0,'ethernet1/2',2,'port1'],[1,'right',3,'port1'],[2,'port2',3,'port1']];
  for (const [a, aInterface, b, bInterface] of links) {
    await page.locator('#network-add-link').click();
    const row = page.locator('.network-link-row').last();
    await row.locator('.network-link-a-node').selectOption(ids[a]);
    await row.locator('.network-link-a-interface').fill(aInterface);
    await row.locator('.network-link-b-node').selectOption(ids[b]);
    await row.locator('.network-link-b-interface').fill(bInterface);
  }
  await page.locator('#network-source-node').selectOption(ids[0]);
  await page.locator('#network-source-interface').fill('ethernet1/1');
  await page.locator('#network-destination-node').selectOption(ids[3]);
  await page.locator('#network-destination-interface').fill('port2');
  await page.locator('#network-flow-source').fill('10.1.0.42');
  await page.locator('#network-flow-destination').fill('203.0.113.15');
  await page.locator('#network-flow-protocol').selectOption('tcp');
  await page.locator('#network-flow-source-port').fill('52000');
  await page.locator('#network-flow-destination-port').fill('443');
  await page.locator('#network-explore').click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  await expect(page.locator('#exploration-branch option')).toHaveCount(2);
  await expect(page.locator('#exploration-disposition')).toContainText('INDETERMINATE');
  const evidence = page.locator('#exploration-forward-hops .evidence-link').first();
  await evidence.click();
  await expect(page.locator('#source-caption')).toContainText('branch-edge');
  await expect(page.locator('#source-text')).not.toBeEmpty();
  await page.locator('#close-source').click();
  const [download] = await Promise.all([page.waitForEvent('download'), page.locator('#exploration-export').click()]);
  const original = await readDownload(download);
  expect(original.request.nodes.map((node) => node.snapshot_id)).toEqual(set.members.map((member) => member.snapshot_id));

  await page.reload();
  await expect(page.locator('#workspace-title')).toHaveText('Enterprise network operations');
  await expect(page.locator('#active-set-name')).toHaveText(setName);
  await expect(page.locator('#active-set-site')).toContainText('Chicago DC');
  await page.locator('#exploration-history-list .exploration-history-open').first().click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  const [replayedDownload] = await Promise.all([page.waitForEvent('download'), page.locator('#exploration-export').click()]);
  expect(await readDownload(replayedDownload)).toEqual(original);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator('#workspace-home').screenshot({ path: path.join(screenshots, 'workspace-imports-mobile.png') });
  for (const width of [320,390,768,1440]) {
    await page.setViewportSize({ width, height: 900 });
    const bounds = await page.evaluate(() => ({
      width: document.documentElement.scrollWidth,
      overflowing: [...document.querySelectorAll('body *')].map((element) => {
        const box = element.getBoundingClientRect();
        return { id: element.id, className: element.getAttribute('class'), left: box.left, right: box.right, width: box.width };
      }).filter((box) => box.width && box.right > window.innerWidth + 1).slice(0, 25),
    }));
    expect(bounds.width, JSON.stringify(bounds.overflowing)).toBeLessThanOrEqual(width);
  }
});

test('invalid and duplicate files preserve the published baseline and sanitized preview never renders credentials', async ({ page }) => {
  await enterWorkspace(page);
  const existing = await workspace(page);
  const config = (await sample(page)).configs.find((item) => item.vendor === 'cisco-asa');
  const raw = `${config.config}\nusername preview-user password PREVIEW_SECRET_CANARY_5817\n`;
  await page.locator('#batch-file-input').setInputFiles([
    { name: 'valid.asa.cfg', mimeType: 'text/plain', buffer: Buffer.from(raw) },
    { name: 'invalid.cfg', mimeType: 'text/plain', buffer: Buffer.from('not a supported firewall configuration') },
  ]);
  await validate(page, 2);
  await expect(page.locator('#batch-preview-results .preview-item[data-status="invalid"]')).toHaveCount(1);
  await expect(page.locator('#batch-preview-results')).not.toContainText('PREVIEW_SECRET_CANARY_5817');
  await page.locator('#batch-set-name').fill('Rejected batch');
  await expect(page.locator('#batch-publish')).toBeDisabled();
  expect((await workspace(page)).workspace.active_set_id).toBe(existing.workspace.active_set_id);
  await page.locator('#batch-items .batch-remove').last().click();
  await expect(page.locator('#batch-preview-results')).toBeEmpty();
  await expect(page.locator('#batch-publish')).toBeDisabled();
  await page.locator('#batch-file-input').setInputFiles({ name: 'same-content.cfg', mimeType: 'text/plain', buffer: Buffer.from(raw) });
  await validate(page, 2);
  await expect(page.locator('#batch-preview-results .preview-item[data-status="duplicate"]')).toHaveCount(1);
  await expect(page.locator('#batch-publish')).toBeDisabled();
  expect((await workspace(page)).workspace.active_set_id).toBe(existing.workspace.active_set_id);
});

test('publishes an unassigned set, switches baselines and persists active selection', async ({ page }, info) => {
  await enterWorkspace(page);
  let prior = await workspace(page);
  let priorSet = prior.snapshot_sets.find((set) => set.id === prior.workspace.active_set_id);
  if (!priorSet) {
    await page.locator('#batch-load-sample').click();
    await expect(page.locator('#batch-items .batch-item')).toHaveCount(4);
    await validate(page, 4);
    await publish(page, `Switching baseline ${info.retry}`);
    prior = await workspace(page);
    priorSet = prior.snapshot_sets.find((set) => set.id === prior.workspace.active_set_id);
  }
  expect(priorSet).toBeTruthy();
  const config = (await sample(page)).configs[3];
  await page.locator('#batch-file-input').setInputFiles({ name: 'renamed-core.conf', mimeType: 'text/plain', buffer: Buffer.from(config.config) });
  await validate(page, 1);
  await page.locator('#batch-site-select').selectOption('');
  const set = await publish(page, `Unassigned core ${info.retry}`);
  expect(set.site_id).toBe('');
  expect(set.members[0].name).toBe('renamed-core.conf');
  await page.locator('#snapshot-set-list').selectOption(priorSet.id);
  await expect(page.locator('#active-set-name')).toHaveText(priorSet.name);
  expect((await workspace(page)).workspace.active_set_id).toBe(priorSet.id);
  await page.reload();
  await expect(page.locator('#active-set-name')).toHaveText(priorSet.name);
});

test('discards a preview response when the queued format changes', async ({ page }) => {
  await enterWorkspace(page);
  await page.locator('#batch-load-sample').click();
  await expect(page.locator('#batch-items .batch-item')).toHaveCount(4);
  let release, signal;
  const arrived = new Promise((resolve) => { signal = resolve; });
  const released = new Promise((resolve) => { release = resolve; });
  await page.route('**/api/import-previews', async (route) => {
    const response = await route.fetch();
    signal();
    await released;
    await route.fulfill({ response });
  });
  await page.locator('#batch-preview').click();
  await arrived;
  await page.locator('#batch-items select').first().evaluate((input) => {
    input.disabled = false;
    input.value = 'auto';
    input.dispatchEvent(new Event('change', { bubbles: true }));
  });
  release();
  await expect(page.locator('#batch-status')).toContainText(/changed during validation/i);
  await expect(page.locator('#batch-preview-results')).toBeEmpty();
  await expect(page.locator('#batch-publish')).toBeDisabled();
});
