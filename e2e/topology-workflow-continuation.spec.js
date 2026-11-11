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
  return seen;
}

async function applyFilters(page, seen, action, predicate) {
  const before = seen.projections.length;
  await action();
  await expect.poll(() => seen.projections.slice(before).some(predicate), { timeout: 10_000 }).toBe(true);
  return seen.projections.at(-1);
}

async function exploreBranchSample(page, extraEdgePorts = 0) {
  let analyses = 0;
  let exploration;
  if (extraEdgePorts) {
    await page.route('**/api/sample/network-exploration', async (route) => {
      const response = await route.fetch();
      const sample = await response.json();
      const edge = sample.configs.find((device) => device.node_id === 'edge');
      expect(edge).toBeTruthy();
      const extraInterfaces = Array.from({ length: extraEdgePorts }, (_, offset) => {
        const index = offset + 3;
        return `set network interface ethernet ethernet1/${index} layer3 ip 10.250.0.${index}/24`;
      }).join('\n');
      // Keep the route-selected interface after the synthetic fan-out so a path jump must page to it.
      edge.config = `${extraInterfaces}\n${edge.config.replaceAll('ethernet1/2', 'ethernet1/99')}`;
      sample.links.forEach((link) => {
        for (const endpoint of [link.a, link.b]) {
          if (endpoint.node_id === 'edge' && endpoint.interface === 'ethernet1/2') endpoint.interface = 'ethernet1/99';
        }
      });
      await route.fulfill({ response, json: sample });
    });
  }
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
  return { exploration, analyses: () => analyses };
}

async function snapshotForNode(page, nodeId) {
  const snapshotId = await page.locator('.network-node-row').evaluateAll((rows, id) => {
    const row = rows.find((item) => item.querySelector('.network-node-id')?.value === id);
    return row?.querySelector('.network-snapshot-id')?.value || null;
  }, nodeId);
  expect(snapshotId).toMatch(/^[a-f0-9]{64}$/);
  const body = await page.evaluate(async (id) => (await fetch(`/api/snapshots/${encodeURIComponent(id)}`)).json(), snapshotId);
  expect(body.snapshot).toBeTruthy();
  return body.snapshot;
}

test('scope edits stay drafts until Apply commits a Rust projection over the full synthetic inventory', async ({ page }) => {
  const seen = await loadLayerLab(page);
  expect(seen.projections.at(-1).counts).toMatchObject({ hosts: 1000 });
  await applyFilters(page, seen, () => page.locator('#tl-tab-l2').click(), (projection) => projection.view === 'l2');

  const priorRequests = seen.requests.length;
  await page.clock.install();
  await page.locator('#tl-vrf').fill('lab-vrf-01');
  await page.locator('#tl-vdom').fill('campus');
  await expect(page.locator('#tl-filter-count')).toContainText(/pending|Apply/i);
  await expect(page.locator('#tl-apply-filters')).toBeEnabled();
  await expect(page.locator('#tl-active-filters .tl-filter-chip')).toHaveCount(0);
  expect(seen.requests).toHaveLength(priorRequests);
  expect(seen.projections.at(-1).filters).toMatchObject({ vrf: '', vdom: '' });
  await page.locator('#tl-vrf').fill('');
  await page.locator('#tl-vdom').fill('');
  await expect(page.locator('#tl-filter-count')).toHaveText('No active filters');
  await expect(page.locator('#tl-apply-filters')).toBeDisabled();
  expect(seen.requests).toHaveLength(priorRequests);
  await page.locator('#tl-vrf').fill('lab-vrf-01');
  await page.locator('#tl-vdom').fill('campus');
  await page.clock.runFor(220);
  await expect.poll(() => seen.projections.length).toBeGreaterThan(priorRequests);
  await expect(page.locator('#tl-filter-count')).toHaveText('2 active filters');
  const requestsAfterAutoApply = seen.requests.length;

  const scoped = seen.projections.at(-1);
  expect(scoped.filters).toMatchObject({ vrf: 'lab-vrf-01', vdom: 'campus' });
  expect(seen.requests.at(-1)).toMatchObject({ vrf: 'lab-vrf-01', vdom: 'campus', view: 'l2' });
  await page.locator('#tl-vrf').press('Tab');
  await page.clock.runFor(300);
  expect(seen.requests).toHaveLength(requestsAfterAutoApply);
  expect(scoped.counts).toMatchObject({ hosts: 1000, matched_hosts: 100 });
  expect(scoped.hosts).toHaveLength(25);
  await expect(page.locator('#tl-host-results-summary')).toContainText('of 100 matching hosts');
  await expect(page.locator('#tl-page-label')).toContainText('Hosts 1–25 of 100');
  await page.locator('#tl-next').click();
  await expect.poll(() => seen.projections.at(-1).pagination.offset).toBe(25);
  expect(seen.projections.at(-1).hosts[0].id).toBe('host-0026');
  await page.locator('#tl-first').click();
  await expect.poll(() => seen.projections.at(-1).pagination.offset).toBe(0);

  // An explicit source is pinned across context edits; destination and the old trace are discarded.
  await page.locator('.tl-host-result[data-host-id="host-0001"] [data-action="source"]').click();
  await page.locator('.tl-host-result[data-host-id="host-0002"] [data-action="destination"]').click();
  await expect(page.locator('#tl-source')).toHaveValue(/host-0001/);
  await expect(page.locator('#tl-destination')).toHaveValue(/host-0002/);
  await page.locator('#tl-vrf').fill('lab-vrf-02');
  const changed = await applyFilters(page, seen, () => page.locator('#tl-apply-filters').click(),
    (projection) => projection.filters.vrf === 'lab-vrf-02');
  expect(changed.filters.vdom).toBe('campus');
  await expect(page.locator('#tl-source')).toHaveValue(/host-0001/);
  await expect(page.locator('#tl-destination')).toHaveValue('');
  expect(changed.counts).toMatchObject({ hosts: 1000, matched_hosts: 0 });
  await expect(page.locator('#tl-host-results-summary')).toContainText('No hosts match');
  const recovered = await applyFilters(page, seen,
    () => page.locator('#tl-active-filters [data-filter-kind="vdom"]').click(),
    (projection) => projection.filters.vrf === 'lab-vrf-02' && projection.filters.vdom === '' && projection.counts.matched_hosts === 100);
  expect(recovered.hosts).toHaveLength(25);
  await expect(page.locator('#tl-vdom')).toHaveValue('');
  await page.locator('.tl-host-result[data-host-id="host-0101"] [data-action="destination"]').click();
  const unavailable = await applyFilters(page, seen, () => page.locator('#tl-trace-button').click(),
    (projection) => projection.path?.reason === 'outside_selected_context');
  expect(unavailable.path).toMatchObject({ status: 'indeterminate', hops: [], links: [], evidence: [] });
  await expect(page.locator('#tl-source')).toHaveValue(/host-0001/);
  await expect(page.locator('#tl-trace-status')).toContainText(/outside the current VRF\/VDOM and filter context/i);
});

test('a late projection response cannot erase filter-draft status', async ({ page }) => {
  const seen = await loadLayerLab(page);
  let releaseResponse;
  let capturedResponse;
  const intercepted = { requests: [], projections: [] };
  const held = new Promise((resolve) => { releaseResponse = resolve; });
  const captured = new Promise((resolve) => { capturedResponse = resolve; });
  let heldOnce = false;
  let released = false;
  await page.route('**/api/topology-layers', async (route) => {
    const request = route.request().postDataJSON();
    const response = await route.fetch();
    const body = await response.json();
    const projection = body.projection || body;
    intercepted.requests.push(request);
    intercepted.projections.push(projection);
    if (!heldOnce && route.request().method() === 'POST') {
      heldOnce = true;
      capturedResponse();
      await held;
      released = true;
    }
    await route.fulfill({ response, json: body });
  });
  await page.clock.install();
  await page.locator('#tl-tab-l2').click();
  await captured;
  await page.locator('#tl-vrf').fill('lab-vrf-01');
  await expect(page.locator('#tl-filter-count')).toContainText('pending');
  expect(intercepted.requests.at(-1).view).toBe('l2');
  expect(intercepted.requests.at(-1).vrf).toBe('');

  releaseResponse();
  await expect.poll(() => released).toBe(true);
  await expect(page.locator('#tl-inspector')).toHaveAttribute('aria-busy', 'false');
  await expect(page.locator('#tl-filter-count')).toContainText('pending');
  await expect(page.locator('#tl-active-filters .tl-filter-chip')).toHaveCount(0);
  expect(intercepted.projections.length).toBeGreaterThan(0);
  expect(intercepted.projections[0].view).toBe('l2');

  await page.clock.runFor(220);
  await expect.poll(() => intercepted.projections.some((projection) => projection.filters.vrf === 'lab-vrf-01')).toBe(true);
  await expect(page.locator('#tl-filter-count')).toHaveText('1 active filter');
});

test('focused firewall port remains discoverable across bounded pages with path and evidence ownership intact', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  const { exploration, analyses } = await exploreBranchSample(page, 25);
  const branches = exploration.result.branches;
  const selection = branches.flatMap((branch, branchIndex) => branch.returns.map((returned, returnIndex) => ({ branch, branchIndex, returnIndex, returned })))
    .find(({ branch, returned }) => branch.forward.hops.some((hop) => hop.node_id === 'edge')
      && returned.direction.hops.some((hop) => hop.node_id === 'edge'));
  expect(selection).toBeTruthy();
  await page.locator('#topology-branch-select').selectOption(String(selection.branchIndex));
  await page.locator('#topology-return-select').selectOption(String(selection.returnIndex));
  const edge = await snapshotForNode(page, 'edge');
  expect(edge.path.interfaces.length).toBeGreaterThanOrEqual(27);
  const edgeNames = edge.path.interfaces.map((iface) => iface.name || iface.physical);
  expect(new Set(edgeNames).size).toBe(edgeNames.length);
  const forwardEdgeHop = selection.branch.forward.hops.find((hop) => hop.node_id === 'edge');
  const routePort = forwardEdgeHop.path.egress_interface;
  expect(routePort).toBe('ethernet1/99');
  expect(edgeNames.indexOf(routePort)).toBeGreaterThanOrEqual(24);

  await page.locator('#topology-device-list [data-node-id="edge"]').click();
  const pathGroup = page.locator('#topology-inspector details[data-group="path"]');
  if (!(await pathGroup.evaluate((element) => element.open))) await pathGroup.locator('summary').click();
  await expect(pathGroup.locator('.topology-hop-card[data-hop-direction="forward"][data-hop-node-id="edge"]')).toBeVisible();
  await expect(pathGroup.locator('.topology-hop-card[data-hop-direction="return"][data-hop-node-id="edge"]')).toBeVisible();

  // Navigate from a peer link to the path-selected port, which is on page three.
  await page.locator('#topology-device-list [data-node-id="north"]').click();
  let topologyGroup = page.locator('#topology-inspector details[data-group="topology"]');
  if (!(await topologyGroup.evaluate((element) => element.open))) await topologyGroup.locator('summary').click();
  const northPort = page.locator('.topology-port-detail[data-node-id="north"][data-port-detail="left"]');
  await northPort.locator('.topology-port-toggle').click();
  await northPort.locator('.topology-neighbor-link[data-link-index]').first().click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'link');
  await page.locator(`#topology-inspector .topology-endpoint-button[data-interface="${routePort}"]`).click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'device');
  topologyGroup = page.locator('#topology-inspector details[data-group="topology"]');
  await expect(topologyGroup).toHaveAttribute('open', '');
  const routePortDetail = page.locator(`#topology-inspector .topology-port-detail[data-port-detail="${routePort}"]`);
  const pager = page.locator('.topology-port-finder-results .topology-port-pager');
  await expect(pager.locator('.topology-port-page-count')).toHaveText('25–27 of 27 returned interfaces');
  await expect(routePortDetail.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');
  await expect(routePortDetail.locator('.topology-hop-card[data-hop-direction="forward"][data-hop-node-id="edge"]')).toBeVisible();

  const history = page.locator('#topology-inspector .topology-inspector-history');
  await history.locator('[data-history="back"]').click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'link');
  await history.locator('[data-history="forward"]').click();
  await expect(routePortDetail.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');

  await pager.getByRole('button', { name: 'Previous' }).click();
  await expect(pager.locator('.topology-port-page-count')).toContainText('13–24 of 27');
  await expect(pager.getByRole('button', { name: 'Previous' })).toBeFocused();
  await pager.getByRole('button', { name: 'Previous' }).click();
  await expect(pager.locator('.topology-port-page-count')).toContainText('1–12 of 27');
  await expect(pager.getByRole('button', { name: 'Previous' })).toBeDisabled();
  await expect(pager.getByRole('button', { name: 'Next' })).toBeFocused();
  const firstPageNames = await page.locator('.topology-port-finder-results .topology-port-detail').evaluateAll((ports) => ports.map((port) => port.dataset.portDetail));
  expect(firstPageNames).toHaveLength(12);
  await pager.getByRole('button', { name: 'Next' }).click();
  await expect(pager.locator('.topology-port-page-count')).toContainText('13–24 of 27');
  await expect(pager.getByRole('button', { name: 'Next' })).toBeFocused();
  const secondPageNames = await page.locator('.topology-port-finder-results .topology-port-detail').evaluateAll((ports) => ports.map((port) => port.dataset.portDetail));
  expect(secondPageNames).toHaveLength(12);
  expect(secondPageNames.some((name) => !firstPageNames.includes(name))).toBe(true);
  await pager.getByRole('button', { name: 'Next' }).click();
  await expect(pager.locator('.topology-port-page-count')).toHaveText('25–27 of 27 returned interfaces');
  await expect(pager.getByRole('button', { name: 'Next' })).toBeDisabled();
  await expect(pager.getByRole('button', { name: 'Previous' })).toBeFocused();
  await expect(routePortDetail.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');

  const restored = routePortDetail;
  await expect(restored).toBeVisible();
  await expect(restored.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');
  const evidenceButton = restored.locator('.topology-source-button').first();
  await expect(evidenceButton).toBeVisible();
  const ownership = await evidenceButton.evaluate((button) => ({ snapshotId: button.dataset.snapshotId, evidenceRef: button.dataset.evidenceRef }));
  const snapshot = await page.evaluate(async (id) => (await fetch(`/api/snapshots/${id}`)).json(), ownership.snapshotId);
  expect(snapshot.snapshot.evidence.some((item) => item.id === ownership.evidenceRef)).toBe(true);
  await evidenceButton.click();
  await expect(page.locator('#source-caption')).toContainText(snapshot.snapshot.hostname);
  await expect(page.locator('#source-caption')).toContainText(ownership.snapshotId.slice(0, 12));
  const viewport = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth }));
  expect(viewport).toEqual({ viewport: 375, document: 375 });
  expect(analyses()).toBe(1);
});
