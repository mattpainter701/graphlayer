const { test, expect } = require('@playwright/test');

async function observeLayerApi(page) {
  const observed = { samples: [], projections: [] };
  await page.route('**/api/sample/topology-layers', async (route) => {
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    observed.samples.push(await response.json());
    await route.fulfill({ response });
  });
  await page.route('**/api/topology-layers', async (route) => {
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    observed.projections.push(await response.json());
    await route.fulfill({ response });
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

async function expectNoHorizontalOverflow(page, width) {
  const sizes = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  expect(sizes.viewport).toBe(width);
  expect(sizes.document, JSON.stringify(sizes)).toBeLessThanOrEqual(width);
  expect(sizes.body, JSON.stringify(sizes)).toBeLessThanOrEqual(width);
}

test('synthetic network map pages hosts, preserves global counts, and keeps presentation actions local', async ({ page }) => {
  const observed = await observeLayerApi(page);
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto('/#network-map');
  await expect(page.locator('#topology-layers')).toBeVisible();
  await page.locator('#tl-load').click();
  await expect.poll(() => observed.samples.length).toBe(1);
  await expect.poll(() => observed.projections.length).toBeGreaterThan(0);

  const dataset = observed.samples[0].dataset || observed.samples[0];
  expect(dataset.synthetic).toBe(true);
  expect(dataset.hosts).toHaveLength(1000);
  expect(dataset.hosts[999].id).toBe('host-1000');

  await waitForProjection(page, observed, () => page.locator('#tl-tab-l2').click(), (projection) => projection.view === 'l2');
  let projection = latestProjection(observed);
  expect(projection.pagination).toMatchObject({ offset: 0, limit: 25, total: 1000 });
  expect(projection.counts).toMatchObject({ hosts: 1000, matched_hosts: 1000 });
  expect(projection.hosts).toHaveLength(25);
  expect(projection.hosts[0].id).toBe('host-0001');

  for (const [size, expectedLength] of [[50, 50], [100, 100]]) {
    await waitForProjection(page, observed, () => page.locator('#tl-page-size').selectOption(String(size)), (next) => next.pagination && next.pagination.limit === size);
    projection = latestProjection(observed);
    expect(projection.pagination).toMatchObject({ offset: 0, limit: size, total: 1000 });
    expect(projection.counts).toMatchObject({ hosts: 1000, matched_hosts: 1000 });
    expect(projection.hosts).toHaveLength(expectedLength);
    expect(projection.nodes.length).toBeLessThanOrEqual(140);
  }
  await page.locator('#tl-next').click();
  await expect.poll(() => latestProjection(observed).pagination.offset).toBe(100);
  projection = latestProjection(observed);
  expect(projection.pagination).toMatchObject({ limit: 100, total: 1000 });
  expect(projection.hosts[0].id).toBe('host-0101');
  expect(projection.counts.hosts).toBe(1000);

  await page.locator('#tl-page-size').selectOption('25');
  await expect.poll(() => latestProjection(observed).pagination.offset).toBe(0);
  await page.locator('#tl-network').fill('net-02');
  await page.locator('#tl-network').dispatchEvent('change');
  await expect.poll(() => latestProjection(observed).pagination.total).toBe(100);
  expect(latestProjection(observed).counts.hosts).toBe(1000);
  await page.locator('#tl-search').fill('host-1000');
  await expect.poll(() => latestProjection(observed).pagination.total).toBe(0);
  expect(latestProjection(observed).hosts).toHaveLength(0);
  expect(latestProjection(observed).counts.hosts).toBe(1000);
  await page.locator('#tl-network').fill('');
  await page.locator('#tl-network').dispatchEvent('change');
  await expect.poll(() => latestProjection(observed).pagination.total).toBe(1);
  expect(latestProjection(observed).hosts.map((host) => host.id)).toEqual(['host-1000']);
  expect(latestProjection(observed).counts.hosts).toBe(1000);
  await page.locator('#tl-reset-filters').click();
  await expect.poll(() => latestProjection(observed).pagination.total).toBe(1000);
  await expect(page.locator('#tl-network')).toHaveValue('');
  await expect(page.locator('#tl-zone')).toHaveValue('');
  await expect(page.locator('#tl-search')).toHaveValue('');
  expect(latestProjection(observed).counts.hosts).toBe(1000);

  const firstHost = page.getByRole('button', { name: 'host: host-0001' });
  await expect(firstHost).toBeVisible();
  await firstHost.focus();
  await page.keyboard.press('Enter');
  await expect(page.locator('#tl-inspector')).toContainText('host-0001');
  await expect(page.locator('#tl-inspector .tl-details')).toContainText('10.20.1.1');
  await expect(page.locator('#tl-inspector .tl-connections')).toContainText(/Connections · [1-9]/);
  await page.locator('#tl-evidence-button').click();
  await expect(page.locator('#tl-sources')).toHaveAttribute('open', '');
  await expect(page.locator('#tl-source-body')).toContainText(/SHA-256 [a-f0-9]{64}/);
  const evidence = dataset.hosts[0].evidence[0];
  const source = dataset.sources.find((item) => item.id === evidence.source_id);
  expect(source).toBeTruthy();
  expect(source.lines.some((line) => line.number === evidence.line && line.text.includes(dataset.hosts[0].ip))).toBe(true);

  await page.locator('#tl-fullscreen').click();
  await expect(page.locator('#topology-layers')).toHaveClass(/tl-fullscreen/);
  await expect(page.locator('#tl-fullscreen')).toHaveAttribute('aria-pressed', 'true');
  await expect(page.locator('#tl-inspector')).toBeVisible();
  await expect(page.locator('#tl-inspector')).toHaveAttribute('aria-label', 'Selection details');
  await expect(page.locator('#tl-inspector')).toHaveAttribute('aria-hidden', 'false');
  await expect(page.locator('#tl-inspector')).toContainText('host-0001');
  const fullscreenLayout = await page.evaluate(() => {
    const graph = document.querySelector('#tl-graph');
    const inspector = document.querySelector('#tl-inspector');
    const graphBox = graph.getBoundingClientRect();
    const inspectorBox = inspector.getBoundingClientRect();
    const hit = document.elementFromPoint(graphBox.left + graphBox.width / 2, graphBox.top + graphBox.height / 2);
    return {
      graphVisible: graphBox.width > 0 && graphBox.height > 0,
      inspectorVisible: inspectorBox.width > 0 && inspectorBox.height > 0,
      overlap: graphBox.left < inspectorBox.right && graphBox.right > inspectorBox.left
        && graphBox.top < inspectorBox.bottom && graphBox.bottom > inspectorBox.top,
      graphOwnsCenterHit: Boolean(hit && graph.contains(hit)),
    };
  });
  expect(fullscreenLayout.graphVisible).toBe(true);
  expect(fullscreenLayout.inspectorVisible).toBe(true);
  expect(fullscreenLayout.overlap, JSON.stringify(fullscreenLayout)).toBe(false);
  expect(fullscreenLayout.graphOwnsCenterHit, JSON.stringify(fullscreenLayout)).toBe(true);
  await page.locator('#tl-fullscreen').click();
  await expect(page.locator('#topology-layers')).not.toHaveClass(/tl-fullscreen/);
  await expect(page.locator('#tl-inspector')).toContainText('host-0001');

  const projectionCount = observed.projections.length;
  const before = await page.locator('#tl-stage').getAttribute('transform');
  const graph = page.locator('#tl-graph');
  await graph.focus();
  await expect(graph).toBeFocused();
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => graph.locator('#tl-stage').getAttribute('transform')).not.toBe(before);
  await page.keyboard.press('Shift+=');
  await expect(page.locator('#tl-zoom')).toContainText('%');
  await page.locator('#tl-fit').click();
  await expect.poll(() => graph.locator('#tl-stage').getAttribute('transform')).toBe('translate(0 0) scale(1)');

  const box = await graph.boundingBox();
  const beforePointerPan = await page.locator('#tl-stage').getAttribute('transform');
  const panStarted = Date.now();
  await page.mouse.move(box.x + box.width * 0.97, box.y + box.height * 0.03);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * 0.86, box.y + box.height * 0.82, { steps: 6 });
  await page.mouse.up();
  const panDurationMs = Date.now() - panStarted;
  expect(panDurationMs).toBeLessThan(5000);
  await expect.poll(() => page.locator('#tl-stage').getAttribute('transform')).not.toBe(beforePointerPan);
  await expect(page.locator('#tl-inspector')).toContainText('host-0001');
  expect(observed.projections).toHaveLength(projectionCount);

  await page.setViewportSize({ width: 375, height: 812 });
  await expectNoHorizontalOverflow(page, 375);
  await firstHost.focus();
  await expect(firstHost).toBeFocused();
  await page.setViewportSize({ width: 1280, height: 900 });
  await expectNoHorizontalOverflow(page, 1280);
});

test('topology workbench controls pan and zoom the real sample without rerunning analysis', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto('http://127.0.0.1:8089/#network-path');
  let analysisPosts = 0;
  await page.route('**/api/network-explorations', async (route) => {
    if (route.request().method() === 'POST') analysisPosts += 1;
    await route.continue();
  });
  await page.locator('#network-load-exploration-sample').click();
  await expect(page.locator('#network-status')).toContainText(/4 sample firewall snapshots imported/i);
  await expect(page.locator('#topology-stage')).toBeVisible();
  await expect(page.locator('#topology-device-list .topology-device')).toHaveCount(6);
  await page.locator('#network-explore').click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  await expect(page.locator('#topology-minimap')).toBeVisible();
  await expect(page.locator('#topology-overview')).toContainText('Devices');
  await expect(page.locator('#topology-overview')).toContainText('Declared links');
  await expect.poll(() => analysisPosts).toBe(1);

  const svgPresentation = await page.locator('#exploration-topology').evaluate((svg) => {
    const icon = getComputedStyle(svg.querySelector('.topology-device-icon path'));
    const pill = getComputedStyle(svg.querySelector('.topology-port-label rect'));
    const label = getComputedStyle(svg.querySelector('.topology-port-label text'));
    const uncertain = getComputedStyle(svg.querySelector('.topology-edge.topology-state-indeterminate'));
    return { iconFill: icon.fill, iconStroke: icon.stroke, pillFill: pill.fill, labelAnchor: label.textAnchor, uncertainStroke: uncertain.stroke };
  });
  expect(svgPresentation.iconFill).toBe('none');
  expect(svgPresentation.iconStroke).not.toBe('none');
  expect(svgPresentation.pillFill).toBe('rgb(255, 255, 255)');
  expect(svgPresentation.labelAnchor).toBe('middle');
  expect(svgPresentation.uncertainStroke).toBe('rgb(192, 139, 50)');
  for (const direction of ['forward', 'return']) {
    await page.locator(`#topology-show-${direction}`).uncheck();
    await expect(page.locator('#exploration-topology .topology-node:visible')).toHaveCount(6);
    const overlay = page.locator(`#exploration-topology .topology-overlays .topology-${direction}`).first();
    await expect(overlay).toHaveCSS('display', 'none');
    await page.locator(`#topology-show-${direction}`).check();
    await expect(overlay).not.toHaveCSS('display', 'none');
  }

  await page.locator('#topology-search').fill('north');
  await expect(page.locator('#topology-device-list .topology-device')).toHaveCount(1);
  await page.locator('#topology-filter-reset').click();
  await expect(page.locator('#topology-search')).toHaveValue('');
  await expect(page.locator('#topology-device-list .topology-device')).toHaveCount(6);
  const north = page.locator('#exploration-topology .topology-node[data-node-id="north"]');
  await north.focus();
  await north.press('Enter');
  await expect(page.locator('#topology-inspector')).toContainText('north');

  await page.locator('#topology-fullscreen').click();
  const fullscreenLayout = await page.evaluate(() => {
    const overview = document.querySelector('#topology-overview').getBoundingClientRect();
    const values = [...document.querySelectorAll('#topology-overview strong')].map((item) => item.getBoundingClientRect());
    const graph = document.querySelector('#topology-stage').getBoundingClientRect();
    const inspector = document.querySelector('#topology-inspector').getBoundingClientRect();
    return {
      overviewValuesFit: values.every((value) => value.top >= overview.top && value.bottom <= overview.bottom),
      graphHeight: graph.height,
      overlap: graph.left < inspector.right && graph.right > inspector.left && graph.top < inspector.bottom && graph.bottom > inspector.top,
    };
  });
  expect(fullscreenLayout.overviewValuesFit, JSON.stringify(fullscreenLayout)).toBe(true);
  expect(fullscreenLayout.graphHeight).toBeGreaterThanOrEqual(300);
  expect(fullscreenLayout.overlap, JSON.stringify(fullscreenLayout)).toBe(false);
  await page.locator('#topology-fullscreen').click();
  await expect(page.locator('#topology-inspector')).toContainText('north');

  const stage = page.locator('#topology-stage');
  await stage.focus();
  await expect(stage).toBeFocused();
  const fitted = await page.locator('#exploration-topology').evaluate((svg) => ({ x: svg.viewBox.baseVal.x, y: svg.viewBox.baseVal.y, width: svg.viewBox.baseVal.width, height: svg.viewBox.baseVal.height }));
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => page.locator('#exploration-topology').evaluate((svg) => svg.viewBox.baseVal.x)).not.toBe(fitted.x);
  await page.keyboard.press('Shift+=');
  await expect.poll(() => page.locator('#topology-zoom').textContent()).not.toBe('100%');
  await expect(page.locator('#topology-inspector')).toContainText('north');
  await page.keyboard.press('0');
  await expect(page.locator('#topology-zoom')).toHaveText('100%');
  await page.locator('#topology-minimap').focus();
  await expect(page.locator('#topology-minimap')).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.locator('#topology-zoom')).toHaveText('100%');
  await page.locator('#topology-fit').click();
  await expect(page.locator('#topology-zoom')).toHaveText('100%');
  await expect(page.locator('#topology-inspector')).toContainText('north');
  expect(analysisPosts).toBe(1);

  await page.setViewportSize({ width: 375, height: 812 });
  await expectNoHorizontalOverflow(page, 375);
  await expect(page.getByRole('button', { name: 'Center topology overview' })).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 900 });
  await expectNoHorizontalOverflow(page, 1280);
});
