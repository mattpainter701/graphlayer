const { test, expect } = require('@playwright/test');

test.use({ baseURL: 'http://127.0.0.1:8089' });

async function loadLayerLab(page) {
  const seen = { dataset: null, requests: [], projections: [] };
  await page.route('**/api/sample/topology-layers', async (route) => {
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    const body = await response.json();
    seen.dataset = body.dataset || body;
    await route.fulfill({ response });
  });
  await page.route('**/api/topology-layers', async (route) => {
    seen.requests.push(route.request().postDataJSON());
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    const body = await response.json();
    seen.projections.push(body.projection || body);
    await route.fulfill({ response });
  });
  await page.goto('/#network-map');
  await page.locator('#tl-load').click();
  await expect.poll(() => seen.projections.length).toBeGreaterThan(0);
  expect(seen.dataset.synthetic).toBe(true);
  expect(seen.dataset.hosts).toHaveLength(1000);
  expect(seen.projections.at(-1).counts.hosts).toBe(1000);
  return seen;
}

async function waitForProjection(page, seen, action, predicate) {
  const before = seen.projections.length;
  await action();
  await expect.poll(() => seen.projections.slice(before).some(predicate), { timeout: 10_000 }).toBe(true);
  return seen.projections.at(-1);
}

async function loadBranchSample(page) {
  let exploration;
  let analyses = 0;
  await page.route('**/api/network-explorations', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    analyses += 1;
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    exploration = (await response.json()).exploration;
    await route.fulfill({ response });
  });
  await page.goto('/#network-path');
  await page.locator('#network-load-exploration-sample').click();
  await expect(page.locator('#network-status')).toContainText('4 sample firewall snapshots imported');
  await page.locator('#network-explore').click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  await expect.poll(() => analyses).toBe(1);
  return exploration;
}

async function loadBranchSampleWithOmittedProjectedPort(page, interfaceName) {
  let capturedProjection;
  await page.route('**/api/topology-view', async (route) => {
    const request = route.request().postDataJSON();
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    const body = await response.json();
    if (request.exploration_id) {
      const edgeNode = body.topology.nodes.find((node) => node.node_id === 'edge');
      expect(edgeNode).toBeTruthy();
      const originalInterface = edgeNode.interfaces.find((iface) => (iface.name || iface.physical) === interfaceName);
      expect(originalInterface).toBeTruthy();
      const originalTopology = structuredClone(body.topology);
      edgeNode.interfaces = edgeNode.interfaces.filter((iface) => (iface.name || iface.physical) !== interfaceName);
      capturedProjection = { request, originalTopology, returnedTopology: structuredClone(body.topology) };
    }
    await route.fulfill({ response, json: body });
  });
  const exploration = await loadBranchSample(page);
  await expect.poll(() => capturedProjection).toBeTruthy();
  return { exploration, capturedProjection };
}

async function snapshotForNode(page, nodeId) {
  const snapshotId = await page.locator('.network-node-row').evaluateAll((rows, id) => {
    const row = rows.find((item) => item.querySelector('.network-node-id')?.value === id);
    return row?.querySelector('.network-snapshot-id')?.value || null;
  }, nodeId);
  expect(snapshotId).toMatch(/^[a-f0-9]{64}$/);
  const response = await page.evaluate(async (id) => (await fetch(`/api/snapshots/${encodeURIComponent(id)}`)).json(), snapshotId);
  expect(response.snapshot).toBeTruthy();
  return response.snapshot;
}

test('removing a scope chip commits its cleared value and the remaining filter draft in one full-inventory request', async ({ page }) => {
  const seen = await loadLayerLab(page);
  const scopedHost = seen.dataset.hosts.find((host) => host.vrf === 'lab-vrf-01' && host.vdom === 'campus' && host.zone);
  expect(scopedHost).toBeTruthy();
  const zone = scopedHost.zone;

  await waitForProjection(page, seen, () => page.locator('#tl-tab-l2').click(), (projection) => projection.view === 'l2');
  const mapItems = page.locator('#tl-stage .tl-node, #tl-stage .tl-edge');
  await expect(mapItems.first()).toBeVisible();
  await mapItems.first().focus();
  await page.keyboard.press('ArrowDown');
  await expect(mapItems.nth(1)).toBeFocused();
  await page.locator('#tl-vrf').fill('lab-vrf-01');
  await page.locator('#tl-vdom').fill('campus');
  await page.locator('#tl-search').fill('host-');
  const initialScope = await waitForProjection(page, seen, () => page.locator('#tl-apply-filters').click(),
    (projection) => projection.filters.vrf === 'lab-vrf-01' && projection.filters.vdom === 'campus' && projection.filters.search === 'host-');
  expect(initialScope.counts).toMatchObject({ hosts: 1000, matched_hosts: 100 });
  await expect(page.locator('#tl-active-filters .tl-filter-chip')).toHaveCount(3);
  const scopedHosts = seen.dataset.hosts.filter((host) => host.vrf === 'lab-vrf-01' && host.vdom === 'campus' && host.id.includes('host-'));
  expect(initialScope.hosts.map((host) => host.id)).toEqual(scopedHosts.slice(0, 25).map((host) => host.id));
  await page.locator('#tl-next').click();
  await expect.poll(() => seen.projections.at(-1).pagination.offset).toBe(25);
  expect(seen.projections.at(-1).hosts.map((host) => host.id)).toEqual(scopedHosts.slice(25, 50).map((host) => host.id));
  await page.locator('#tl-first').click();
  await expect.poll(() => seen.projections.at(-1).pagination.offset).toBe(0);

  await page.clock.install({ time: new Date('2026-10-27T18:00:00Z') });
  await page.clock.pauseAt(new Date('2026-10-27T18:00:01Z'));
  const previousRequestCount = seen.requests.length;
  await page.locator('#tl-zone').fill(zone);
  await expect(page.locator('#tl-filter-count')).toContainText('pending');
  expect(seen.requests).toHaveLength(previousRequestCount);
  await expect(page.locator('#tl-active-filters .tl-filter-chip')).toHaveCount(3);

  const committed = await waitForProjection(page, seen,
    () => page.locator('#tl-active-filters [data-filter-kind="search"]').click(),
    (projection) => projection.filters.vrf === 'lab-vrf-01'
      && projection.filters.vdom === 'campus' && projection.filters.search === '' && projection.filters.zone === zone);
  expect(seen.requests).toHaveLength(previousRequestCount + 1);
  expect(seen.requests.at(-1)).toMatchObject({ view: 'l2', vrf: 'lab-vrf-01', vdom: 'campus', search: '', zone });

  const matchingHosts = seen.dataset.hosts.filter((host) => host.vrf === 'lab-vrf-01' && host.vdom === 'campus' && host.zone === zone);
  expect(matchingHosts.length).toBeGreaterThan(0);
  expect(committed.counts).toMatchObject({ hosts: 1000, matched_hosts: matchingHosts.length });
  expect(committed.pagination).toMatchObject({ limit: 25, total: matchingHosts.length, offset: 0 });
  expect(committed.hosts.map((host) => host.id)).toEqual(matchingHosts.slice(0, 25).map((host) => host.id));
  await expect(page.locator('#tl-vrf')).toHaveValue('lab-vrf-01');
  await expect(page.locator('#tl-vdom')).toHaveValue('campus');
  await expect(page.locator('#tl-zone')).toHaveValue(zone);
  await expect(page.locator('#tl-search')).toBeFocused();
  await expect(page.locator('#tl-active-filters .tl-filter-chip')).toHaveCount(3);

  await page.clock.runFor(300);
  expect(seen.requests).toHaveLength(previousRequestCount + 1);
  const beforeSecondChip = seen.requests.length;
  await page.locator('#tl-search').fill(scopedHost.id);
  await expect(page.locator('#tl-filter-count')).toContainText('pending');
  const narrowed = await waitForProjection(page, seen,
    () => page.locator('#tl-active-filters [data-filter-kind="vdom"]').click(),
    (projection) => projection.filters.vrf === 'lab-vrf-01' && projection.filters.vdom === ''
      && projection.filters.zone === zone && projection.filters.search === scopedHost.id);
  expect(seen.requests).toHaveLength(beforeSecondChip + 1);
  expect(narrowed.counts).toMatchObject({ hosts: 1000, matched_hosts: 1 });
  await expect(page.locator('#tl-vdom')).toHaveValue('');
  await expect(page.locator('#tl-search')).toHaveValue(scopedHost.id);
  await expect(page.locator('#tl-vdom')).toBeFocused();
  await page.clock.runFor(300);
  expect(seen.requests).toHaveLength(beforeSecondChip + 1);
});

test('following a filtered link endpoint reveals and focuses the real returned port through history and breadcrumbs', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  const exploration = await loadBranchSample(page);
  expect(exploration.request.links.length).toBeGreaterThan(0);
  const edge = await snapshotForNode(page, 'edge');
  expect(edge.path.interfaces.map((iface) => iface.name || iface.physical)).toContain('ethernet1/2');

  await page.locator('#topology-device-list [data-node-id="edge"]').click();
  const topology = page.locator('#topology-inspector details[data-group="topology"]');
  if (!(await topology.evaluate((element) => element.open))) await topology.locator('summary').click();
  const finder = page.locator('.topology-port-finder-input');
  await finder.fill('ethernet1/1');
  await expect(page.locator('.topology-port-detail[data-port-detail="ethernet1/2"]')).toHaveCount(0);

  // The link remains discoverable from the selected device while the port list is filtered.
  await page.locator('#topology-inspector .topology-neighbor-link[data-link-index]').first().click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'link');
  await page.getByRole('button', { name: 'Inspect interface ethernet1/2 on edge', exact: true }).click();
  const port = page.locator('#topology-inspector .topology-port-detail[data-node-id="edge"][data-port-detail="ethernet1/2"]');
  await expect(port).toBeVisible();
  await expect(port.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');
  await expect(port.locator('.topology-port-toggle')).toBeFocused();
  await expect(finder).toHaveValue('ethernet1/1');
  expect(await port.locator('.topology-port-detail-body').innerText()).toMatch(/ethernet1\/2|172\.16\.0\.1/);

  const history = page.locator('#topology-inspector .topology-inspector-history');
  await history.locator('[data-history="back"]').click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'link');
  await history.locator('[data-history="forward"]').click();
  await expect(port.locator('.topology-port-toggle')).toBeFocused();
  const portBreadcrumb = page.locator('#topology-inspector .topology-breadcrumb-button[data-port-name="ethernet1/2"]').last();
  await portBreadcrumb.click();
  await expect(port.locator('.topology-port-toggle')).toBeFocused();
  const evidenceButton = port.locator('.topology-source-button').first();
  await expect(evidenceButton).toBeVisible();
  const evidenceRef = await evidenceButton.evaluate((button) => ({
    snapshotId: button.dataset.snapshotId,
    ref: button.dataset.evidenceRef,
  }));
  expect(evidenceRef.snapshotId).toBe(edge.id);
  expect(edge.evidence.some((item) => item.id === evidenceRef.ref)).toBe(true);
  await evidenceButton.click();
  await expect(page.locator('#source-caption')).toContainText(edge.hostname);
  await expect(page.locator('#source-caption')).toContainText(edge.id.slice(0, 12));
  const viewport = await page.evaluate(() => ({ width: innerWidth, document: document.documentElement.scrollWidth }));
  expect(viewport).toEqual({ width: 375, document: 375 });
});

test('partial topology projection omission stays safe while the valid saved port remains evidence-owned', async ({ page }) => {
  const interfaceName = 'ethernet1/2';
  const { exploration, capturedProjection } = await loadBranchSampleWithOmittedProjectedPort(page, interfaceName);
  const edge = await snapshotForNode(page, 'edge');
  const savedInterface = edge.path.interfaces.find((iface) => (iface.name || iface.physical) === interfaceName);
  expect(savedInterface).toBeTruthy();
  expect(savedInterface.evidence_refs.length).toBeGreaterThan(0);
  expect(capturedProjection.request.exploration_id).toBe(exploration.id);
  const originalEdge = capturedProjection.originalTopology.nodes.find((node) => node.node_id === 'edge');
  const returnedEdge = capturedProjection.returnedTopology.nodes.find((node) => node.node_id === 'edge');
  expect(originalEdge.interfaces.some((iface) => (iface.name || iface.physical) === interfaceName)).toBe(true);
  expect(returnedEdge.interfaces.some((iface) => (iface.name || iface.physical) === interfaceName)).toBe(false);
  expect(returnedEdge).toEqual({
    ...originalEdge,
    interfaces: originalEdge.interfaces.filter((iface) => (iface.name || iface.physical) !== interfaceName),
  });
  expect(capturedProjection.returnedTopology.links).toEqual(capturedProjection.originalTopology.links);
  expect(exploration.request.links.some((link) => (link.a.node_id === 'edge' && link.a.interface === interfaceName)
    || (link.b.node_id === 'edge' && link.b.interface === interfaceName))).toBe(true);

  await page.locator('#topology-device-list [data-node-id="edge"]').click();
  await page.locator('#topology-inspector .topology-neighbor-link[data-link-index]').first().click();
  const linkInspector = page.locator('#topology-inspector .topology-inspector-content[data-inspector-kind="link"]');
  await expect(linkInspector).toContainText(interfaceName);
  await linkInspector.getByRole('button', { name: `Inspect interface ${interfaceName} on edge`, exact: true }).click();

  const unavailable = page.locator('#topology-inspector .topology-port-unavailable[role="status"]');
  await expect(unavailable).toBeVisible();
  await expect(unavailable).toContainText(`No matching projected interface facts were returned for declared link endpoint ${interfaceName} on edge; port configuration and path details are unavailable.`);
  await expect(unavailable).toBeFocused();
  await expect(page.locator(`#topology-inspector .topology-port-detail[data-port-detail="${interfaceName}"]`)).toHaveCount(0);

  const history = page.locator('#topology-inspector .topology-inspector-history');
  await history.locator('[data-history="back"]').click();
  await expect(linkInspector).toBeVisible();
  await expect(linkInspector).toContainText(interfaceName);
  await history.locator('[data-history="forward"]').click();
  await expect(unavailable).toBeVisible();
  await expect(unavailable).toBeFocused();
  await expect(page.locator(`#topology-inspector .topology-port-detail[data-port-detail="${interfaceName}"]`)).toHaveCount(0);
});
