const { test, expect } = require('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');

const screenshotRoot = process.env.GRAPHLAYER_E2E_SCREENSHOT_DIR || path.resolve('test-results');

async function observeLayerApi(page) {
  const observed = { samples: [], projections: [], failures: [], csrf: '', allowFailure: false };
  await page.route('**/api/sample/topology-layers', async (route) => {
    observed.csrf = route.request().headers()['x-csrf-token'] || '';
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    observed.samples.push(await response.json());
    await route.fulfill({ response });
  });
  await page.route('**/api/topology-layers', async (route) => {
    observed.csrf = route.request().headers()['x-csrf-token'] || observed.csrf;
    const response = await route.fetch();
    const payload = await response.json();
    if (response.ok()) observed.projections.push(payload);
    else {
      observed.failures.push({ status: response.status(), payload });
      if (!observed.allowFailure) expect(response.ok()).toBe(true);
    }
    await route.fulfill({ response });
  });
  return observed;
}

async function workspaceState(page) {
  return page.evaluate(async () => {
    const paths = ['/api/snapshots', '/api/investigations', '/api/path-investigations'];
    const responses = await Promise.all(paths.map(async (url) => {
      const response = await fetch(url, { credentials: 'same-origin' });
      if (!response.ok) throw new Error(`Workspace state request failed: ${url} (${response.status})`);
      return response.json();
    }));
    return responses.map((body) => body.snapshots || body.investigations || []);
  });
}

async function rejectProjection(page, observed, input) {
  observed.allowFailure = true;
  try {
    return await page.evaluate(async ({ body, csrf }) => {
      const response = await fetch('/api/topology-layers', { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf }, body: JSON.stringify(body) });
      return { status: response.status, payload: await response.json() };
    }, { body: input, csrf: observed.csrf });
  } finally {
    observed.allowFailure = false;
  }
}

function projectionPayload(observed, index = observed.projections.length - 1) {
  const result = observed.projections[index];
  return result && (result.projection || result);
}

async function waitForProjection(page, observed, action, predicate = () => true) {
  const prior = observed.projections.length;
  await action();
  await expect.poll(() => observed.projections.slice(prior).map((r) => r.projection || r).some(predicate), { timeout: 10_000 }).toBe(true);
}

async function nodeCount(page) {
  return page.locator('#tl-svg .tl-node').count();
}

async function noNodeOverlaps(page) {
  return page.locator('#tl-svg').evaluate((svg) => {
    const boxes = [...svg.querySelectorAll('.tl-node')].map((node) => {
      const box = node.getBBox();
      const matrix = node.getCTM();
      const points = [[box.x, box.y], [box.x + box.width, box.y], [box.x, box.y + box.height], [box.x + box.width, box.y + box.height]].map(([x, y]) => {
        const point = svg.createSVGPoint(); point.x = x; point.y = y;
        const transformed = point.matrixTransform(matrix);
        return [transformed.x, transformed.y];
      });
      return { left: Math.min(...points.map((p) => p[0])), right: Math.max(...points.map((p) => p[0])), top: Math.min(...points.map((p) => p[1])), bottom: Math.max(...points.map((p) => p[1])) };
    });
    const overlaps = [];
    for (let i = 0; i < boxes.length; i++) for (let j = i + 1; j < boxes.length; j++) {
      const a = boxes[i], b = boxes[j];
      if (a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top) overlaps.push([i, j]);
    }
    return overlaps;
  });
}

test('network map scales through real Rust projections, evidence, tracing, and export', async ({ page }) => {
  const observed = await observeLayerApi(page);
  const diagnostics = {};
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto('/#network-map');
  await expect(page.locator('#topology-layers')).toBeVisible();
  const workspaceBefore = await workspaceState(page);
  const loadStarted = Date.now();
  await page.locator('#tl-load').click();

  await expect.poll(() => observed.samples.length).toBe(1);
  await expect.poll(() => observed.projections.length).toBeGreaterThan(0);
  await expect.poll(() => nodeCount(page)).toBeGreaterThan(0);
  const dataset = observed.samples[0].dataset || observed.samples[0];
  expect(dataset.synthetic).toBe(true);
  expect(dataset.hosts).toHaveLength(1000);
  expect(dataset.hosts[0].id).toBe('host-0001');
  expect(dataset.hosts[999].id).toBe('host-1000');
  expect(dataset.networks).toHaveLength(10);
  expect(dataset.zones).toHaveLength(4);
  expect(dataset.sources.map((source) => source.id)).toEqual(expect.arrayContaining(['cfg-panos', 'cfg-fortios']));
  expect(dataset.parsed_configs.map((config) => config.vendor).sort()).toEqual(['fortinet-fortigate', 'paloalto-panos']);
  expect(observed.csrf).toBeTruthy();

  const l3 = projectionPayload(observed);
  expect(l3.counts).toMatchObject({ hosts: 1000, networks: 10, zones: 4 });
  expect(l3.view).toBe('l3');
  expect(l3.nodes.length).toBeLessThanOrEqual(30);
  expect(l3.nodes.filter((node) => node.role === 'network')).toHaveLength(10);
  expect(await nodeCount(page)).toBeLessThanOrEqual(30);
  diagnostics.loadAndL3RenderMs = Date.now() - loadStarted;
  diagnostics.l3NodeCount = await nodeCount(page);
  expect(diagnostics.loadAndL3RenderMs).toBeLessThanOrEqual(5000);
  await expect(page.locator('#tl-caption')).toContainText(/configuration-derived map/i);
  await expect(page.locator('#topology-layers')).toContainText(/do not establish live reachability/i);
  await fs.mkdir(screenshotRoot, { recursive: true });
  await page.locator('#tl-graph').screenshot({ path: path.join(screenshotRoot, 'topology-layers-l3-graph.png') });

  const switchStarted = Date.now();
  await waitForProjection(page, observed, () => page.locator('#tl-tab-zone').click(), (p) => p.view === 'zone');
  await expect(page.locator('#tl-svg .tl-node')).toHaveCount(4);
  diagnostics.zoneSwitchRenderMs = Date.now() - switchStarted;
  diagnostics.zoneNodeCount = await nodeCount(page);
  expect(diagnostics.zoneSwitchRenderMs).toBeLessThanOrEqual(5000);
  const zones = projectionPayload(observed);
  expect(zones.counts).toMatchObject({ hosts: 1000, networks: 10, zones: 4 });
  expect(zones.nodes).toHaveLength(4);
  expect(zones.nodes.map((node) => node.zone).sort()).toEqual(['engineering', 'operations', 'research', 'shared-services']);
  expect(zones.links.every((link) => link.kind === 'configured_boundary' && link.evidence.length > 0)).toBe(true);
  for (const edge of zones.links) for (const reference of edge.evidence) {
    const source = dataset.sources.find((item) => item.id === reference.source_id);
    expect(source).toBeTruthy();
    expect(reference.ref).toBe(`${source.id}:${source.sha256.slice(0, 12)}:L${reference.line}`);
  }
  await expect(page.locator('#tl-svg .tl-node')).toHaveCount(4);
  await page.getByRole('button', { name: 'zone: engineering' }).click();
  await expect(page.locator('#tl-tab-l3')).toHaveAttribute('aria-selected', 'true');
  await expect(page.locator('#tl-zone')).toHaveValue('engineering');
  await expect.poll(() => projectionPayload(observed).counts.matched_hosts).toBe(300);
  expect(projectionPayload(observed).nodes.length).toBeLessThanOrEqual(30);

  await page.getByRole('button', { name: 'network: 10.20.1.0/24' }).click();
  await expect(page.locator('#tl-tab-l2')).toHaveAttribute('aria-selected', 'true');
  await expect(page.locator('#tl-network')).toHaveValue('net-01');
  await expect.poll(() => projectionPayload(observed).view).toBe('l2');
  expect(projectionPayload(observed).pagination).toMatchObject({ limit: 25, total: 100 });
  expect(projectionPayload(observed).nodes.length).toBeLessThanOrEqual(35);
  await expect.poll(() => nodeCount(page)).toBeLessThan(36);

  await page.locator('#tl-network').fill('');
  await page.locator('#tl-network').dispatchEvent('change');
  await page.locator('#tl-zone').fill('');
  await page.locator('#tl-zone').dispatchEvent('change');
  await expect.poll(() => projectionPayload(observed).pagination.total).toBe(1000);
  expect(projectionPayload(observed).hosts).toHaveLength(25);
  expect(projectionPayload(observed).hosts[0].id).toBe('host-0001');
  expect(await nodeCount(page)).toBeLessThanOrEqual(35);
  await page.locator('#tl-next').click();
  await expect.poll(() => projectionPayload(observed).pagination.offset).toBe(25);
  expect(projectionPayload(observed).hosts).toHaveLength(25);
  expect(projectionPayload(observed).hosts[0].id).toBe('host-0026');
  await expect(page.locator('#tl-page-label')).toContainText('Hosts 26–50 of 1000');
  await expect.poll(() => nodeCount(page)).toBeLessThan(36);
  diagnostics.l2PageNodeCount = await nodeCount(page);

  await page.locator('#tl-search').fill('host-1000');
  await expect.poll(() => projectionPayload(observed).pagination.total).toBe(1);
  expect(projectionPayload(observed).hosts.map((host) => host.id)).toEqual(['host-1000']);
  await expect(page.getByRole('button', { name: 'host: host-1000' })).toBeVisible();
  await expect(page.locator('#tl-page-label')).toContainText('Hosts 1–1 of 1');
  await page.locator('#tl-search').fill('');
  await expect.poll(() => projectionPayload(observed).pagination.total).toBe(1000);

  const firstHost = page.getByRole('button', { name: 'host: host-0001' });
  await firstHost.focus();
  await page.keyboard.press('Enter');
  await expect(page.locator('#tl-inspector')).toContainText('host-0001');
  await expect(page.locator('#tl-inspector .tl-details')).toContainText('10.20.1.1');
  await expect(page.locator('#tl-inspector .tl-details')).toContainText(/02:20:/i);
  await expect(page.locator('#tl-inspector .tl-details')).toContainText('VLAN');
  await expect(page.locator('#tl-inspector .tl-details')).toContainText('sw-01-a');
  await expect(page.locator('#tl-inspector .tl-details')).toContainText(/Gi1\/0\/1/);
  await page.locator('#tl-evidence-button').click();
  await expect(page.locator('#tl-sources')).toHaveAttribute('open', '');
  await expect(page.locator('#tl-source-body')).toContainText(/SHA-256 [a-f0-9]{64}/);
  const hostEvidence = dataset.hosts[0].evidence[0];
  const hostSource = dataset.sources.find((source) => source.id === hostEvidence.source_id);
  const hostEvidenceLine = hostSource.lines.find((line) => line.number === hostEvidence.line).text;
  expect(hostEvidence.ref).toBe(`${hostSource.id}:${hostSource.sha256.slice(0, 12)}:L${hostEvidence.line}`);
  expect(hostEvidenceLine).toContain(dataset.hosts[0].id);
  expect(hostEvidenceLine).toContain(dataset.hosts[0].ip);
  expect(hostEvidenceLine).toContain(dataset.hosts[0].mac);
  await expect(page.locator('#tl-source-body')).toContainText(hostEvidenceLine);

  await page.locator('#tl-source').fill('host-0001 (host-0001)');
  await page.locator('#tl-source').dispatchEvent('change');
  await page.locator('#tl-destination').fill('host-0075 (host-0075)');
  await page.locator('#tl-destination').dispatchEvent('change');
  await waitForProjection(page, observed, () => page.locator('#tl-trace-button').click(), (p) => p.path && p.path.status === 'modeled_l2_path');
  const traced = projectionPayload(observed);
  expect(traced.path.links).toHaveLength(4);
  expect(traced.path.hops.map((hop) => hop.node_id)).toEqual(expect.arrayContaining(['host-0001', 'sw-01-a', 'sw-01-d', 'sw-01-b', 'host-0075']));
  expect(traced.path.evidence.length).toBeGreaterThan(0);
  await expect(page.locator('#tl-trace-status')).toContainText('modeled_l2_path');
  await expect(page.locator('#tl-trace-status')).toContainText('Gi1/0/51');
  await expect(page.locator('#tl-trace-status')).toContainText('VLAN 101');
  await page.locator('#tl-trace-status .tl-hop-source').first().click();
  await expect(page.locator('#tl-source-body')).toContainText('host-0001');
  await expect(page.locator('#tl-svg .tl-on-path')).toHaveCount(5);

  await page.locator('#tl-network').fill('net-02');
  await page.locator('#tl-network').dispatchEvent('change');
  await expect.poll(() => projectionPayload(observed).nodes.some((node) => node.id === 'host-0101')).toBe(true);
  await page.locator('#tl-destination').fill('host-0101 (host-0101)');
  await page.locator('#tl-destination').dispatchEvent('change');
  await waitForProjection(page, observed, () => page.locator('#tl-trace-button').click(), (p) => p.path && p.path.status === 'requires_l3');
  const crossNetwork = projectionPayload(observed);
  expect(crossNetwork.path).toMatchObject({ status: 'requires_l3', reason: 'different_network_or_vlan', hops: [], links: [] });
  await expect(page.locator('#tl-trace-status')).toContainText('requires_l3');
  await expect(page.locator('#tl-svg .tl-on-path')).toHaveCount(0);

  const [download] = await Promise.all([page.waitForEvent('download'), page.locator('#tl-export').click()]);
  expect(download.suggestedFilename()).toBe('graphlayer-synthetic-topology-lab.json');
  const stream = await download.createReadStream(); const chunks = [];
  for await (const chunk of stream) chunks.push(chunk);
  const exported = JSON.parse(Buffer.concat(chunks).toString('utf8'));
  expect(exported.hosts).toHaveLength(1000);
  expect(exported.sources.map((source) => source.id)).toEqual(expect.arrayContaining(['cfg-panos', 'cfg-fortios']));
  expect(exported).toEqual(dataset);

  const invalidSchema = structuredClone(dataset);
  invalidSchema.schema_version = 99;
  const schemaFailure = await rejectProjection(page, observed, { dataset: invalidSchema, view: 'l3' });
  expect(schemaFailure.status).toBe(400);
  expect(schemaFailure.payload.error.code).toBe('invalid_topology_layers');
  const tamperedSource = structuredClone(dataset);
  tamperedSource.sources[0].text += 'tampered evidence line\n';
  const sourceFailure = await rejectProjection(page, observed, { dataset: tamperedSource, view: 'l3' });
  expect(sourceFailure.status).toBe(400);
  expect(sourceFailure.payload.error.code).toBe('invalid_topology_layers');
  expect(observed.failures).toHaveLength(2);

  await page.locator('#tl-load').click();
  await expect.poll(() => observed.samples.length).toBe(2);
  const repeated = observed.samples[1].dataset || observed.samples[1];
  expect(repeated).toEqual(dataset);
  await page.goto('/#network-path');
  await page.goto('/#network-map');
  await expect(page.locator('#tl-svg .tl-node')).not.toHaveCount(0);
  expect(projectionPayload(observed).counts.hosts).toBe(1000);

  await page.setViewportSize({ width: 390, height: 844 });
  const viewport = await page.evaluate(() => ({ width: innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  expect(viewport.document).toBeLessThanOrEqual(390);
  expect(viewport.body).toBeLessThanOrEqual(390);
  expect(await nodeCount(page)).toBeLessThanOrEqual(35);
  expect(await noNodeOverlaps(page)).toEqual([]);
  diagnostics.mobileNodeCount = await nodeCount(page);
  diagnostics.viewport = viewport;
  await page.locator('#tl-graph').screenshot({ path: path.join(screenshotRoot, 'topology-layers-mobile-graph.png') });
  expect(await workspaceState(page)).toEqual(workspaceBefore);
  diagnostics.workspaceEntryCountsBefore = workspaceBefore.map((entries) => entries.length);
  diagnostics.workspaceUnchanged = true;
  await fs.writeFile(path.join(screenshotRoot, 'topology-layers-diagnostics.json'), `${JSON.stringify(diagnostics, null, 2)}\n`, 'utf8');
});
