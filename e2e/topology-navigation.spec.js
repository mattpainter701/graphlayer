const { test, expect } = require('@playwright/test');

async function observeMap(page) {
  const seen = { dataset: null, projections: [], requests: [] };
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
  await expect(page.locator('#tl-inspector')).toHaveAttribute('aria-busy', 'false');
  expect(seen.dataset.hosts).toHaveLength(1000);
  return seen;
}

async function updateMap(page, seen, action, predicate) {
  const before = seen.projections.length;
  await action();
  await expect.poll(() => seen.projections.slice(before).some(predicate), { timeout: 10_000 }).toBe(true);
  await expect(page.locator('#tl-inspector')).toHaveAttribute('aria-busy', 'false');
  return seen.projections.at(-1);
}

async function filter(page, seen, name, value) {
  return updateMap(page, seen, async () => {
    await page.locator(`#tl-${name}`).fill(value);
    await page.locator(`#tl-${name}`).press('Tab');
  }, (projection) => projection.filters[name] === value);
}

const resultRow = (page, id) => page.locator(`.tl-host-result[data-host-id="${id}"]`);

async function explore(page) {
  const seen = { posts: 0, exploration: null };
  await page.route('**/api/network-explorations', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    seen.posts += 1;
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    seen.exploration = (await response.json()).exploration;
    await route.fulfill({ response });
  });
  await page.goto('http://127.0.0.1:8089/#network-path');
  await page.locator('#network-load-exploration-sample').click();
  await expect(page.locator('#network-status')).toContainText('4 sample firewall snapshots imported');
  await page.locator('#network-explore').click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  await expect.poll(() => seen.posts).toBe(1);
  return seen;
}

async function openGroup(page, group) {
  const details = page.locator(`#topology-inspector details[data-group="${group}"]`);
  if (!(await details.evaluate((element) => element.open))) await details.locator('summary').click();
  await expect(details).toHaveAttribute('open', '');
  return details;
}

test('finds an off-page host from Layer 3 and restores the original page and inspector with Back', async ({ page }) => {
  const seen = await observeMap(page);
  await expect(page.locator('#tl-host-results-summary')).toContainText('1000 in dataset');
  await expect(page.locator('.tl-host-result')).toHaveCount(25);
  await updateMap(page, seen, () => page.locator('#tl-last').click(), (p) => p.pagination.offset === 975);
  await expect(page.locator('#tl-page-position')).toHaveText('Page 40 of 40');
  await expect(page.locator('#tl-last')).toBeDisabled();
  await resultRow(page, 'host-0999').locator('[data-action="inspect"]').click();
  await expect(page.locator('#tl-inspector h3')).toHaveText('host-0999');
  const mapped = await updateMap(page, seen,
    () => resultRow(page, 'host-0999').locator('[data-action="map"]').click(),
    (p) => p.view === 'l2' && p.navigation.focus_status === 'visible');
  expect(mapped.navigation.focus_host_id).toBe('host-0999');
  expect(mapped.pagination.offset).toBe(75);
  expect(mapped.nodes.some((node) => node.id === 'host-0999')).toBe(true);
  expect(mapped.filters.network_id).toBe('net-10');
  await expect(page.locator('#tl-inspector h3')).toHaveText('host-0999');
  await expect(page.locator('#tl-breadcrumbs')).toContainText('host-0999');
  await updateMap(page, seen, () => page.locator('#tl-page-size').selectOption('100'), (p) => p.pagination.limit === 100);
  await updateMap(page, seen, () => page.locator('#tl-navigation-back').click(),
    (p) => p.view === 'l3' && p.pagination.offset === 975 && p.pagination.limit === 25);
  await expect(page.locator('#tl-page-size')).toHaveValue('25');
  await expect(page.locator('#tl-page-position')).toHaveText('Page 40 of 40');
  await expect(page.locator('#tl-network')).toHaveValue('');
  await expect(page.locator('#tl-inspector h3')).toHaveText('host-0999');
  await expect(page.locator('#tl-navigation-back')).toBeDisabled();
});

test('shows scoped facet counts and searches inventory facts without matching evidence hashes', async ({ page }) => {
  const seen = await observeMap(page);
  await filter(page, seen, 'vrf', 'lab-vrf-01');
  const scoped = await filter(page, seen, 'vdom', 'campus');
  expect(scoped.counts.matched_hosts).toBe(100);
  expect(scoped.facets.networks.find((item) => item.value === 'net-01').count).toBe(100);
  expect(scoped.facets.zones.find((item) => item.value === 'engineering').count).toBe(100);
  await expect(page.locator('#tl-host-results-summary')).toContainText('of 100 matching hosts');
  await expect(page.locator('#tl-host-results-summary')).toContainText('1000 in dataset');
  await updateMap(page, seen, async () => {
    await page.locator('#tl-network').fill('net-02');
    await page.locator('#tl-network').press('Tab');
  }, (p) => p.filters.network_id === 'net-02' && p.pagination.total === 0);
  await expect(page.locator('.tl-host-result')).toHaveCount(0);
  await expect(page.locator('#tl-first')).toBeDisabled();
  await expect(page.locator('#tl-last')).toBeDisabled();
  await updateMap(page, seen, () => page.locator('[data-filter-kind="network"]').click(), (p) => p.pagination.total === 100);
  const digest = seen.dataset.hosts[0].evidence[0].sha256;
  await updateMap(page, seen, () => page.locator('#tl-search').fill(digest), (p) => p.filters.search === digest && p.pagination.total === 0);
  await expect(page.locator('#tl-host-results-summary')).toContainText('No hosts match');
  await updateMap(page, seen, () => page.locator('#tl-search').fill('HOST-0001'), (p) => p.pagination.total === 1);
  await expect(resultRow(page, 'host-0001')).toBeVisible();
  expect(seen.projections.at(-1).hosts.map((host) => host.id)).toEqual(['host-0001']);
  await expect(page.locator('#tl-vrf')).toHaveValue('lab-vrf-01');
  await expect(page.locator('#tl-vdom')).toHaveValue('campus');
});

test('keeps explicit trace endpoints while finding hosts and drilling through ordered path ports', async ({ page }) => {
  const seen = await observeMap(page);
  await filter(page, seen, 'vrf', 'lab-vrf-01');
  await filter(page, seen, 'vdom', 'campus');
  await updateMap(page, seen, () => page.locator('#tl-search').fill('host-0001'), (p) => p.pagination.total === 1);
  await resultRow(page, 'host-0001').locator('[data-action="source"]').click();
  await updateMap(page, seen, () => page.locator('#tl-search').fill('host-0075'), (p) => p.hosts[0]?.id === 'host-0075');
  await updateMap(page, seen, () => resultRow(page, 'host-0075').locator('[data-action="map"]').click(),
    (p) => p.navigation.focus_host_id === 'host-0075' && p.navigation.focus_status === 'visible');
  await expect(page.locator('#tl-source')).toHaveValue('host-0001 (host-0001)');
  await resultRow(page, 'host-0075').locator('[data-action="destination"]').click();
  const traced = await updateMap(page, seen, () => page.locator('#tl-trace-button').click(),
    (p) => p.path?.status === 'modeled_l2_path');
  expect(traced.path.hops.map((hop) => hop.node_id)).toEqual(['host-0001', 'sw-01-a', 'sw-01-d', 'sw-01-b', 'host-0075']);
  await expect(page.locator('#tl-pager')).toBeHidden();
  const requests = seen.requests.length;
  await page.locator('.tl-hop-link').filter({ hasText: 'Gi1/0/51' }).first().click();
  await expect(page.locator('#tl-inspector')).toContainText('Gi1/0/51');
  await expect(page.locator('#tl-inspector')).toContainText('Gi1/0/1');
  await page.locator('#tl-evidence-button').click();
  await expect(page.locator('#tl-sources')).toHaveAttribute('open', '');
  const sources = await page.locator('#tl-source-body').innerText();
  expect(traced.sources.some((source) => sources.includes(source.sha256))).toBe(true);
  await expect(page.locator('#tl-trace-status')).toContainText('modeled_l2_path');
  expect(seen.requests).toHaveLength(requests);
});

test('navigates device port to link to neighbor port and back with snapshot-owned evidence', async ({ page }) => {
  const seen = await explore(page);
  await page.locator('#topology-device-list [data-node-id="edge"]').click();
  await openGroup(page, 'topology');
  await page.locator('.topology-port-finder-input').fill('ethernet1/2');
  const port = page.locator('.topology-port-detail[data-port-detail="ethernet1/2"][data-node-id="edge"]');
  await port.locator('.topology-port-toggle').click();
  await expect(port.locator('.topology-port-summary')).toContainText('ethernet1/2');
  await expect(port).toContainText('172.16.0.1/29');
  await port.locator('.topology-neighbor-link[data-link-index="0"]').click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'link');
  await page.locator('#topology-inspector .topology-endpoint-button[data-interface="left"]').click();
  const neighbor = page.locator('.topology-port-detail[data-port-detail="left"][data-node-id="north"]');
  await expect(neighbor.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');
  const source = neighbor.locator('.topology-source-button').first();
  const ownership = await source.evaluate((button) => ({ snapshot: button.dataset.snapshotId, ref: button.dataset.evidenceRef }));
  const snapshot = await page.evaluate(async (id) => (await fetch(`/api/snapshots/${id}`)).json(), ownership.snapshot);
  expect(snapshot.snapshot.hostname).toBe('branch-north');
  expect(snapshot.snapshot.evidence.some((item) => item.id === ownership.ref)).toBe(true);
  await source.click();
  await expect(page.locator('#source-caption')).toContainText('branch-north');
  await page.locator('#close-source').click();
  await page.locator('.topology-inspector-history [data-history="back"]').click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'link');
  await page.locator('.topology-inspector-history [data-history="back"]').click();
  await expect(port.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');
  await page.locator('.topology-inspector-history [data-history="forward"]').click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'link');
  expect(seen.posts).toBe(1);
});

test('port finder recovers from empty matches and new analyses discard stale inspector history', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  const seen = await explore(page);
  await page.locator('#topology-device-list [data-node-id="edge"]').click();
  await openGroup(page, 'topology');
  const finder = page.locator('.topology-port-finder-input');
  await finder.fill('missing-synthetic-port');
  await expect(page.locator('.topology-port-finder-results')).toContainText(/no|0/i);
  await page.locator('.topology-port-finder-clear').click();
  const port = page.locator('.topology-port-detail[data-port-detail="ethernet1/2"][data-node-id="edge"]');
  await expect(port).toBeVisible();
  await port.locator('.topology-port-toggle').click();
  await page.locator('.topology-port-finder-input').fill('missing-synthetic-port');
  await expect(port).toBeVisible();
  await expect(port.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');
  await expect(port).toContainText('Configured status · Unavailable');
  await page.locator('#network-flow-source-port').fill('53041');
  await page.locator('#network-explore').click();
  await expect.poll(() => seen.posts).toBe(2);
  await expect(page.locator('#exploration-result')).toContainText('53041');
  await page.locator('#topology-device-list [data-node-id="edge"]').click();
  await expect(page.locator('.topology-inspector-history [data-history="back"]')).toBeDisabled();
  await expect(page.locator('.topology-inspector-history [data-history="forward"]')).toBeDisabled();
  await openGroup(page, 'topology');
  await expect(page.locator('.topology-port-finder-input')).toHaveValue('');
  const sizes = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth }));
  expect(sizes.document).toBe(sizes.viewport);
});

test('bounded host results and navigation fit desktop, tablet and mobile without horizontal overflow', async ({ page }) => {
  const seen = await observeMap(page);
  await updateMap(page, seen, () => page.locator('#tl-page-size').selectOption('100'), (p) => p.pagination.limit === 100);
  await expect(page.locator('.tl-host-result')).toHaveCount(100);
  for (const width of [1280, 768, 375]) {
    await page.setViewportSize({ width, height: 850 });
    await expect(page.locator('#tl-host-results-summary')).toContainText('of 1000 matching hosts');
    const sizes = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
    expect(sizes, `viewport ${width}`).toEqual({ viewport: width, document: width, body: width });
    for (const action of ['inspect', 'map', 'source', 'destination']) {
      const control = resultRow(page, 'host-0001').locator(`[data-action="${action}"]`);
      expect(await control.evaluate((element) => element.getBoundingClientRect().height)).toBeGreaterThanOrEqual(40);
    }
  }
  await page.screenshot({ path: 'test-results/topology-navigation-host-results.png', fullPage: true });
});

test('fullscreen keeps a bounded canvas and clear footer after restoring expanded port history', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  const seen = await explore(page);
  await page.locator('#topology-device-list [data-node-id="edge"]').click();
  await openGroup(page, 'topology');
  await page.locator('.topology-port-detail[data-port-detail="ethernet1/2"] .topology-port-toggle').click();
  await page.locator('#topology-fullscreen').click();
  for (const viewport of [{ width: 1280, height: 720 }, { width: 768, height: 850 }, { width: 375, height: 812 }]) {
    await page.setViewportSize(viewport);
    const port = page.locator('.topology-port-detail[data-port-detail="ethernet1/2"][data-node-id="edge"]');
    await port.locator('.topology-neighbor-link[data-link-index="0"]').click();
    await page.locator('.topology-endpoint-button[data-interface="left"]').click();
    await page.locator('.topology-inspector-history [data-history="back"]').click();
    await page.locator('.topology-inspector-history [data-history="back"]').click();
    await expect(port.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');
    await page.locator('#topology-stage').scrollIntoViewIfNeeded();
    const geometry = await page.locator('.topology-fullscreen-active').evaluate(async (shell) => {
      await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      const stage = shell.querySelector('.topology-stage').getBoundingClientRect();
      const inspector = shell.querySelector('.topology-inspector').getBoundingClientRect();
      const footer = shell.querySelector('.topology-workbench-note').getBoundingClientRect();
      const center = document.elementFromPoint(stage.left + stage.width / 2, stage.top + stage.height / 2);
      return { stageBottom: stage.bottom, inspectorBottom: inspector.bottom, footerTop: footer.top,
        stageCenterPainted: shell.querySelector('.topology-stage').contains(center),
        stageHeight: stage.height, inspectorHeight: inspector.height, panelHeight: shell.clientHeight,
        panelScrollHeight: shell.scrollHeight, viewport: innerWidth, documentWidth: document.documentElement.scrollWidth };
    });
    expect(geometry.stageHeight, JSON.stringify({ viewport, geometry })).toBeGreaterThanOrEqual(300);
    expect(geometry.stageHeight, JSON.stringify({ viewport, geometry })).toBeLessThanOrEqual(viewport.height);
    expect(geometry.stageCenterPainted, JSON.stringify({ viewport, geometry })).toBe(true);
    expect(geometry.inspectorHeight, JSON.stringify({ viewport, geometry })).toBeGreaterThan(100);
    expect(geometry.footerTop, JSON.stringify({ viewport, geometry })).toBeGreaterThanOrEqual(Math.max(geometry.stageBottom, geometry.inspectorBottom) - 1);
    expect(geometry.documentWidth).toBe(geometry.viewport);
  }
  expect(seen.posts).toBe(1);
});
