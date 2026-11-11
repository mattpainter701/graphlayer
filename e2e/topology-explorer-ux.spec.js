const { test, expect } = require('@playwright/test');

async function observeLayerApi(page, onProjection) {
  const observed = { samples: [], projections: [], projectionRequests: [] };
  await page.route('**/api/sample/topology-layers', async (route) => {
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    observed.samples.push(await response.json());
    await route.fulfill({ response });
  });
  await page.route('**/api/topology-layers', async (route) => {
    if (route.request().method() === 'POST') observed.projectionRequests.push(route.request().postDataJSON());
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    observed.projections.push(await response.json());
    if (onProjection) await onProjection({ route, response, request: route.request() });
    else await route.fulfill({ response });
  });
  return observed;
}

function latestProjection(observed) {
  const response = observed.projections.at(-1);
  return response && (response.projection || response);
}

async function waitForProjection(page, observed, action, predicate = () => true) {
  const prior = observed.projections.length;
  await action();
  await expect.poll(() => observed.projections.slice(prior).some((response) => predicate(response.projection || response)), { timeout: 10_000 }).toBe(true);
}

async function loadSyntheticMap(page, observed) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto('/#network-map');
  await expect(page.locator('#topology-layers')).toBeVisible();
  await page.locator('#tl-load').click();
  await expect.poll(() => observed.samples.length).toBe(1);
  await expect.poll(() => observed.projections.length).toBeGreaterThan(0);
  const dataset = observed.samples[0].dataset || observed.samples[0];
  expect(dataset.synthetic).toBe(true);
  expect(dataset.hosts).toHaveLength(1000);
  return dataset;
}

async function loadAndExploreBranchSample(page) {
  let analysisPosts = 0;
  await page.route('**/api/network-explorations', async (route) => {
    if (route.request().method() === 'POST') analysisPosts += 1;
    await route.continue();
  });
  await page.goto('http://127.0.0.1:8089/#network-path');
  await page.locator('#network-load-exploration-sample').click();
  await expect(page.locator('#network-status')).toContainText(/4 sample firewall snapshots imported/i);
  await page.locator('#network-explore').click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  await expect.poll(() => analysisPosts).toBe(1);
  return () => analysisPosts;
}

async function expectTopologyNodeGeometry(page, width) {
  await page.setViewportSize({ width, height: 850 });
  const geometry = await page.locator('#exploration-topology').evaluate((svg) => {
    const nodes = [...svg.querySelectorAll('.topology-node')].filter((node) => getComputedStyle(node).display !== 'none');
    const failures = [];
    for (const node of nodes) {
      const id = node.dataset.nodeId;
      const icon = node.querySelector('.topology-device-icon').getBoundingClientRect();
      const card = node.querySelector('.topology-node-card').getBoundingClientRect();
      const label = node.querySelector('.topology-node-label');
      const subtitle = node.querySelector('.topology-node-subtitle');
      const labelRect = label.getBoundingClientRect();
      const subtitleRect = subtitle.getBoundingClientRect();
      const labelBox = label.getBBox();
      const subtitleBox = subtitle.getBBox();
      const labelAnchorX = Number(label.getAttribute('x'));
      const subtitleAnchorX = Number(subtitle.getAttribute('x'));
      const insideCard = (box) => box.left >= card.left - 1 && box.right <= card.right + 1
        && box.top >= card.top - 1 && box.bottom <= card.bottom + 1;
      const startsAtAnchor = (box, x) => Math.abs(box.x - x) <= 1;
      const leftAnchored = getComputedStyle(label).textAnchor === 'start'
        && getComputedStyle(subtitle).textAnchor === 'start'
        && startsAtAnchor(labelBox, labelAnchorX)
        && startsAtAnchor(subtitleBox, subtitleAnchorX);
      const iconClear = icon.right <= Math.min(labelRect.left, subtitleRect.left) + 1;
      const textInsideCard = insideCard(labelRect) && insideCard(subtitleRect);
      if (!leftAnchored || !iconClear || !textInsideCard) {
        failures.push({ id, leftAnchored, iconClear, textInsideCard,
          icon: { left: icon.left, right: icon.right },
          card: { left: card.left, right: card.right, top: card.top, bottom: card.bottom },
          title: { left: labelRect.left, right: labelRect.right, top: labelRect.top, bottom: labelRect.bottom, bboxX: labelBox.x, anchorX: labelAnchorX },
          subtitle: { left: subtitleRect.left, right: subtitleRect.right, top: subtitleRect.top, bottom: subtitleRect.bottom, bboxX: subtitleBox.x, anchorX: subtitleAnchorX },
        });
      }
    }
    return { nodeCount: nodes.length, failures };
  });
  expect(geometry.nodeCount, `visible topology nodes at ${width}px`).toBeGreaterThan(0);
  expect(geometry.failures, `node label geometry at ${width}px: ${JSON.stringify(geometry.failures)}`).toEqual([]);
}

test('discovered network and zone filters combine into removable chips and recover from no matches', async ({ page }) => {
  const observed = await observeLayerApi(page);
  const dataset = await loadSyntheticMap(page, observed);
  const projection = latestProjection(observed);
  const network = dataset.networks[1].id || dataset.networks[1].network_id;
  const zone = dataset.hosts.find((host) => host.network_id === network)?.zone;
  const firstInNetwork = dataset.hosts.find((host) => host.network_id === network);
  const secondInNetwork = dataset.hosts.find((host) => host.network_id === network && host.id !== firstInNetwork.id);
  expect(network).toBeTruthy();
  expect(zone).toBeTruthy();
  expect(firstInNetwork).toBeTruthy();
  expect(secondInNetwork).toBeTruthy();

  await expect(page.locator('#tl-network-options option').evaluateAll((options) => options.map((option) => option.value))).resolves.toContain(network);
  const discoveredZones = await page.locator('#tl-zone-options option').evaluateAll((options) => options.map((option) => option.value));
  expect(discoveredZones).toContain(zone);
  expect(discoveredZones).not.toContain('false');
  await waitForProjection(page, observed, async () => {
    await page.locator('#tl-network').fill(network);
    await page.locator('#tl-network').dispatchEvent('change');
  }, (next) => next.pagination && next.pagination.total === dataset.hosts.filter((host) => host.network_id === network).length);
  await waitForProjection(page, observed, async () => {
    await page.locator('#tl-zone').fill(zone);
    await page.locator('#tl-zone').dispatchEvent('change');
  }, (next) => next.pagination && next.pagination.total === dataset.hosts.filter((host) => host.network_id === network && host.zone === zone).length);
  await expect(page.locator('#tl-filter-count')).toHaveText('2 active filters');
  await expect(page.locator('#tl-active-filters .tl-filter-chip')).toHaveCount(2);
  expect(latestProjection(observed).counts.hosts).toBe(1000);
  expect(projection.counts.hosts).toBe(1000);

  await page.locator('#tl-active-filters button[aria-label^="Remove zone filter"]').click();
  await expect(page.locator('#tl-filter-count')).toHaveText('1 active filter');
  await expect(page.locator('#tl-zone')).toHaveValue('');
  await expect.poll(() => latestProjection(observed).counts.matched_hosts).toBeGreaterThan(0);
  expect(latestProjection(observed).counts.hosts).toBe(1000);

  const search = page.locator('#tl-search');
  await search.fill('no-such-synthetic-endpoint');
  await expect(page.locator('#tl-stage .tl-node')).toHaveCount(0);
  await expect.poll(() => latestProjection(observed).counts.matched_hosts).toBe(0);
  await expect(page.locator('#tl-filter-count')).toHaveText('2 active filters');
  expect(latestProjection(observed).counts.hosts).toBe(1000);

  // Two quick edits should leave the latest search in both the controls and the real Rust projection.
  await search.fill(firstInNetwork.id);
  await search.fill(secondInNetwork.id);
  await expect.poll(() => latestProjection(observed).pagination.total).toBe(1);
  expect(latestProjection(observed).hosts.map((host) => host.id)).toEqual([secondInNetwork.id]);
  expect(observed.projectionRequests.at(-1).search).toBe(secondInNetwork.id);
  expect(latestProjection(observed).counts.hosts).toBe(1000);

  await page.locator('#tl-reset-filters').click();
  await expect.poll(() => latestProjection(observed).pagination.total).toBe(1000);
  await expect(page.locator('#tl-filter-count')).toHaveText('No active filters');
  await expect(page.locator('#tl-active-filters .tl-filter-chip')).toHaveCount(0);
  await expect(page.locator('#tl-network')).toHaveValue('');
  await expect(page.locator('#tl-zone')).toHaveValue('');
  await expect(page.locator('#tl-search')).toHaveValue('');
  expect(latestProjection(observed).counts.hosts).toBe(1000);
});

test('neighbor drilldown opens the peer and its evidence while canvas presentation stays local', async ({ page }) => {
  const observed = await observeLayerApi(page);
  const dataset = await loadSyntheticMap(page, observed);
  await waitForProjection(page, observed, () => page.locator('#tl-tab-l2').click(), (result) => result.view === 'l2');
  const firstHost = page.getByRole('button', { name: 'host: host-0001' });
  await expect(firstHost).toBeVisible();
  await firstHost.click();
  await expect(page.locator('#tl-inspector')).toContainText('host-0001');
  const neighbor = page.locator('#tl-inspector .tl-link-nav').first();
  await expect(neighbor).toBeVisible();
  const neighborName = (await neighbor.innerText()).replace(/^[←→]\s*/, '').trim();
  await neighbor.click();
  await expect(page.locator('#tl-inspector')).toContainText(neighborName);
  await expect(page.locator('#tl-clear-selection')).toBeVisible();

  const peer = [...dataset.hosts, ...dataset.devices, ...dataset.networks].find((item) => item.label === neighborName || item.id === neighborName || item.ip === neighborName);
  expect(peer, `neighbor ${neighborName} resolves in the synthetic dataset`).toBeTruthy();
  const evidenceButton = page.locator('#tl-evidence-button');
  await expect(evidenceButton).toBeVisible();
  await evidenceButton.click();
  await expect(page.locator('#tl-sources')).toHaveAttribute('open', '');
  await expect(page.locator('#tl-source-body')).toContainText(/SHA-256 [a-f0-9]{64}/);
  const evidence = peer.evidence[0];
  const source = dataset.sources.find((item) => item.id === evidence.source_id);
  expect(source).toBeTruthy();
  expect(source.lines.some((line) => line.number === evidence.line)).toBe(true);

  const requestCount = observed.projectionRequests.length;
  await page.locator('#tl-fit').click();
  await page.locator('#tl-zoom-in').click();
  await page.locator('#tl-fullscreen').click();
  await expect(page.locator('#topology-layers')).toHaveClass(/tl-fullscreen/);
  await expect(page.locator('#tl-inspector')).toContainText(neighborName);

  for (const width of [1280, 768, 375]) {
    await page.setViewportSize({ width, height: 850 });
    const layout = await page.evaluate(() => {
      const graph = document.querySelector('#tl-graph').getBoundingClientRect();
      const inspector = document.querySelector('#tl-inspector').getBoundingClientRect();
      const width = innerWidth;
      return {
        graphVisible: graph.width > 0 && graph.height > 0,
        inspectorVisible: inspector.width > 0 && inspector.height > 0,
        overlap: graph.left < inspector.right && graph.right > inspector.left && graph.top < inspector.bottom && graph.bottom > inspector.top,
        viewportWidth: width,
        documentWidth: document.documentElement.scrollWidth,
      };
    });
    expect(layout.graphVisible, JSON.stringify(layout)).toBe(true);
    expect(layout.inspectorVisible, JSON.stringify(layout)).toBe(true);
    expect(layout.overlap, JSON.stringify(layout)).toBe(false);
    expect(layout.documentWidth, JSON.stringify(layout)).toBeLessThanOrEqual(layout.viewportWidth);
  }
  await page.locator('#tl-fullscreen').click();
  expect(observed.projectionRequests).toHaveLength(requestCount);
  await expect(page.locator('#tl-inspector')).toContainText(neighborName);
});

test('path facets recover an empty canvas and endpoint navigation keeps saved evidence in view', async ({ page }) => {
  const analysisPosts = await loadAndExploreBranchSample(page);
  await expect(page.locator('#topology-active-filters')).toBeAttached();
  for (const width of [1280, 768, 375]) await expectTopologyNodeGeometry(page, width);
  await page.setViewportSize({ width: 1280, height: 900 });
  const zone = page.locator('#topology-filters select[data-filter="zone"]');
  const vdom = page.locator('#topology-filters select[data-filter="vdom"]');
  await expect(zone).toBeVisible();
  await expect(vdom).toBeVisible();

  await zone.selectOption('trust');
  await vdom.selectOption('root');
  await expect(page.locator('#topology-filter-count')).toContainText('2 active filters');
  await expect(page.locator('#topology-active-filters .topology-filter-chip')).toHaveCount(2);
  await expect(page.locator('#topology-empty-state')).toBeVisible();
  await page.locator('#topology-empty-reset').click();
  await expect(page.locator('#topology-empty-state')).toBeHidden();
  await expect(page.locator('#topology-filter-count')).toContainText('All topology facets');

  await zone.selectOption('trust');
  await expect(page.locator('#topology-device-list .topology-device')).toHaveCount(1);
  await page.locator('#topology-device-list .topology-device[data-node-id="edge"]').click();
  const northNeighbor = page.locator('#topology-inspector .topology-neighbor-button[data-node-id="north"]');
  await expect(northNeighbor).toBeVisible();
  await northNeighbor.click();
  await expect(page.locator('#topology-inspector')).toContainText('north');
  await expect(page.locator('#topology-filter-count')).toContainText('All topology facets');
  await expect(page.locator('#topology-caption')).toContainText(/Adjusted filters to reveal north/i);
  await expect(page.locator('#exploration-topology .topology-node[data-node-id="north"]')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Inspect sources', exact: true })).toBeVisible();

  const inspectLink = page.locator('#topology-inspector .topology-neighbor-row .topology-neighbor-link[data-link-index]').first();
  await expect(inspectLink).toBeVisible();
  await inspectLink.click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'link');
  const northEndpoint = page.locator('#topology-inspector .topology-endpoint-button[data-node-id="north"]');
  await expect(northEndpoint).toBeVisible();
  await northEndpoint.click();
  await expect(page.locator('#topology-inspector')).toContainText('north');
  await page.locator('#topology-inspector details[data-group="evidence"] summary').click();
  const sourceEvidence = page.locator('#topology-inspector details[data-group="evidence"] .topology-source-button').first();
  await expect(sourceEvidence).toBeVisible();
  await sourceEvidence.click();
  await expect(page.locator('#source-lines')).toBeVisible();
  await expect(page.locator('#source-caption')).toContainText('north');
  expect(analysisPosts()).toBe(1);
});

test('search invalidates an older in-flight projection and chip removal cancels a queued search', async ({ page }) => {
  let releaseOldResponse;
  let oldResponseReady;
  let oldResponseReleased = false;
  const oldResponseGate = new Promise((resolve) => { releaseOldResponse = resolve; });
  const oldResponseHeld = new Promise((resolve) => { oldResponseReady = resolve; });
  let heldSearches = 0;
  const observed = await observeLayerApi(page, async ({ route, response, request }) => {
    const body = request.postDataJSON();
    if (body.search === 'host-0001' && heldSearches++ === 0) {
      oldResponseReady();
      await oldResponseGate;
      try {
        await route.fulfill({ response });
      } catch (_) {
        // The UI may abort the old request after the newer search starts.
      }
      oldResponseReleased = true;
      return;
    }
    await route.fulfill({ response });
  });
  const dataset = await loadSyntheticMap(page, observed);
  await waitForProjection(page, observed, () => page.locator('#tl-tab-l2').click(), (projection) => projection.view === 'l2');
  const network = dataset.hosts.find((host) => host.id === 'host-0001').network_id;
  await waitForProjection(page, observed, async () => {
    await page.locator('#tl-network').fill(network);
    await page.locator('#tl-network').dispatchEvent('change');
  }, (projection) => projection.pagination.total === dataset.hosts.filter((host) => host.network_id === network).length);
  await page.getByRole('button', { name: 'host: host-0001' }).click();
  await expect(page.locator('#tl-inspector')).toContainText('host-0001');

  const search = page.locator('#tl-search');
  await search.fill('host-0001');
  await oldResponseHeld;
  await expect.poll(() => observed.projectionRequests.filter((request) => request.search === 'host-0001')).toHaveLength(1);
  await search.fill('host-0002');
  await expect(page.locator('#tl-stage .tl-node')).toHaveCount(0);
  await expect(page.locator('#tl-inspector')).not.toContainText('host-0001');
  await expect.poll(() => observed.projectionRequests.some((request) => request.search === 'host-0002')).toBe(true);
  await expect(page.getByRole('button', { name: 'host: host-0002' })).toBeVisible();
  await expect(page.locator('#tl-inspector')).not.toContainText('host-0001');

  releaseOldResponse();
  await expect.poll(() => oldResponseReleased).toBe(true);
  await expect(page.getByRole('button', { name: 'host: host-0002' })).toBeVisible();
  expect(observed.projectionRequests.at(-1).search).toBe('host-0002');

  // Advancing the browser clock makes the 220 ms debounce deterministic without a wall-clock sleep.
  await page.clock.install();
  const searchInput = page.locator('#tl-search');
  await searchInput.fill('host-0001');
  const requestCount = observed.projectionRequests.length;
  const priorProjectionCount = observed.projections.length;
  await page.locator('#tl-active-filters button[aria-label^="Remove network filter"]').click();
  await expect.poll(() => observed.projectionRequests.length).toBe(requestCount + 1);
  await expect.poll(() => observed.projectionRequests.at(-1)).toMatchObject({ search: 'host-0001', network_id: '' });
  await expect.poll(() => observed.projections.length).toBeGreaterThan(priorProjectionCount);
  await page.clock.runFor(300);
  expect(observed.projectionRequests).toHaveLength(requestCount + 1);
  await expect(page.getByRole('button', { name: 'host: host-0001' })).toBeVisible();
});

test('host inspector endpoint buttons preserve the chosen source through a real L2 trace', async ({ page }) => {
  const observed = await observeLayerApi(page);
  const dataset = await loadSyntheticMap(page, observed);
  await waitForProjection(page, observed, () => page.locator('#tl-tab-l2').click(), (projection) => projection.view === 'l2');

  await page.getByRole('button', { name: 'host: host-0001' }).click();
  await page.locator('#tl-inspector').getByRole('button', { name: 'Use as source' }).click();
  await expect(page.locator('#tl-source')).toHaveValue('host-0001 (host-0001)');

  await page.getByRole('button', { name: 'host: host-0002' }).click();
  await page.locator('#tl-inspector').getByRole('button', { name: 'Use as destination' }).click();
  await expect(page.locator('#tl-source')).toHaveValue('host-0001 (host-0001)');
  await expect(page.locator('#tl-destination')).toHaveValue('host-0002 (host-0002)');

  await waitForProjection(page, observed, () => page.locator('#tl-trace-button').click(), (projection) => projection.path && projection.path.status === 'modeled_l2_path');
  const request = observed.projectionRequests.at(-1);
  expect(request).toMatchObject({ source_host_id: 'host-0001', destination_host_id: 'host-0002' });
  const traced = latestProjection(observed);
  expect(traced.path.status).toBe('modeled_l2_path');
  const hopIds = traced.path.hops.map((hop) => hop.node_id);
  expect(hopIds).toEqual(expect.arrayContaining(['host-0001', 'host-0002']));
  expect(traced.path.evidence.length).toBeGreaterThan(0);

  const sourceHop = traced.path.hops.find((hop) => hop.node_id === 'host-0001');
  expect(sourceHop.evidence.length).toBeGreaterThan(0);
  const sourceRef = sourceHop.evidence[0];
  const source = dataset.sources.find((item) => item.id === sourceRef.source_id);
  expect(source).toBeTruthy();
  const sourceLine = source.lines.find((line) => line.number === sourceRef.line);
  expect(sourceLine).toBeTruthy();
  await page.locator('#tl-trace-status .tl-hop-source[aria-label="Source evidence for host-0001"]').click();
  await expect(page.locator('#tl-sources')).toHaveAttribute('open', '');
  await expect(page.locator('#tl-source-body')).toContainText(`SHA-256 ${source.sha256}`);
  await expect(page.locator('#tl-source-body')).toContainText(sourceLine.text);
});
