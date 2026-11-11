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
  await expect(page.locator('#tl-minimap')).toBeHidden();
  await page.locator('#tl-load').click();
  await expect.poll(() => seen.projections.length).toBeGreaterThan(0);
  await expect(page.locator('#tl-inspector')).toHaveAttribute('aria-busy', 'false');
  expect(seen.dataset.synthetic).toBe(true);
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

const items = (page) => page.locator('#tl-stage .tl-node, #tl-stage .tl-edge');

test('graph keyboard navigation is a single tab stop and reconciles after page and filter redraws', async ({ page }) => {
  const seen = await observeMap(page);
  const initialRequestCount = seen.requests.length;
  await expect(items(page)).not.toHaveCount(0);
  await expect(page.locator('#tl-focus-selection')).toBeDisabled();
  expect(await items(page).evaluateAll((nodes) => nodes.filter((node) => node.getAttribute('tabindex') === '0'))).toHaveLength(1);

  const first = items(page).first();
  await first.focus();
  await page.keyboard.press('ArrowDown');
  const second = items(page).nth(1);
  await expect(second).toBeFocused();
  await page.keyboard.press('ArrowUp');
  await expect(first).toBeFocused();
  await page.keyboard.press('End');
  await expect(items(page).last()).toBeFocused();
  await page.keyboard.press('Home');
  await expect(first).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(page.locator('#tl-graph')).toBeFocused();
  expect(seen.requests).toHaveLength(initialRequestCount);

  await updateMap(page, seen, async () => {
    const network = page.locator('#tl-stage .tl-kind-network').first();
    await network.focus();
    await page.keyboard.press('Enter');
  }, (p) => p.view === 'l2');
  const currentRover = page.locator('#tl-stage .tl-node[tabindex="0"], #tl-stage .tl-edge[tabindex="0"]');
  await expect(currentRover).toHaveCount(1);
  await expect(currentRover).toBeFocused();
  const focusState = await page.evaluate(() => {
    const stage = document.querySelector('#tl-stage');
    const active = document.activeElement;
    return { connected: Boolean(active?.isConnected), insideStage: stage.contains(active), onBody: active === document.body };
  });
  expect(focusState).toEqual({ connected: true, insideStage: true, onBody: false });

  await updateMap(page, seen, async () => {
    await page.locator('#tl-network').fill('');
    await page.locator('#tl-network').press('Tab');
  }, (p) => p.filters.network_id === '');
  await updateMap(page, seen, () => page.locator('#tl-page-size').selectOption('100'), (p) => p.pagination.limit === 100);
  expect(await items(page).evaluateAll((nodes) => nodes.filter((node) => node.getAttribute('tabindex') === '0'))).toHaveLength(1);
  const host = page.locator('#tl-stage .tl-node').filter({ hasText: 'host-0001' });
  await host.focus();
  await page.keyboard.press('Enter');
  await expect(page.locator('#tl-inspector')).toContainText('host-0001');
  await expect(page.locator('#tl-focus-selection')).toBeEnabled();
  await updateMap(page, seen, async () => {
    await page.locator('#tl-vrf').fill('lab-vrf-02');
    await page.locator('#tl-vrf').press('Tab');
  }, (p) => p.filters.vrf === 'lab-vrf-02');
  expect(await items(page).evaluateAll((nodes) => nodes.filter((node) => node.getAttribute('tabindex') === '0'))).toHaveLength(1);
  await expect(page.locator('#tl-focus-selection')).toBeDisabled();
  await expect(page.locator('#tl-inspector')).not.toContainText('host-0001');

  await updateMap(page, seen, async () => {
    await page.locator('#tl-vrf').fill('missing-synthetic-context');
    await page.locator('#tl-vrf').press('Tab');
  }, (p) => p.filters.vrf === 'missing-synthetic-context');
  await expect(items(page)).toHaveCount(0);
  await expect(page.locator('#tl-focus-selection')).toBeDisabled();
});

test('Focus selection makes a selected host readable on desktop, tablet, and mobile without reanalysis', async ({ page }) => {
  const seen = await observeMap(page);
  await updateMap(page, seen, () => page.locator('#tl-tab-l2').click(), (p) => p.view === 'l2');
  const projection = await updateMap(page, seen, () => page.locator('#tl-page-size').selectOption('100'), (p) => p.pagination.limit === 100);
  expect(projection.hosts).toHaveLength(100);
  expect(projection.counts.hosts).toBe(1000);
  expect(await page.locator('#tl-svg .tl-node').count()).toBeLessThanOrEqual(140);

  const target = page.locator('#tl-svg .tl-node').filter({ hasText: 'host-0001' }).first();
  expect(await target.evaluate((node) => node.__item.id)).toBe('host-0001');
  await target.click();
  await expect(page.locator('#tl-focus-selection')).toBeEnabled();
  const requestCount = seen.requests.length;
  for (const [width, height] of [[1280, 900], [768, 850], [375, 812]]) {
    await page.setViewportSize({ width, height });
    await page.locator('#tl-focus-selection').click();
    const measureTarget = () => target.evaluate((node) => {
      const card = node.querySelector('.tl-node-box').getBoundingClientRect();
      const title = node.querySelector('.tl-node-title');
      const titleHeight = title.getBoundingClientRect().height;
      const graph = document.querySelector('#tl-graph').getBoundingClientRect();
      return {
        cardWidth: card.width,
        cardHeight: card.height,
        titleHeight,
        centerX: card.left + card.width / 2,
        centerY: card.top + card.height / 2,
        graphCenterX: graph.left + graph.width / 2,
        graphCenterY: graph.top + graph.height / 2,
      };
    });
    await expect.poll(async () => {
      const metrics = await measureTarget();
      return metrics.cardWidth >= 44 && metrics.cardHeight >= 44 && metrics.titleHeight >= 9
        && Math.abs(metrics.centerX - metrics.graphCenterX) <= 28
        && Math.abs(metrics.centerY - metrics.graphCenterY) <= 28;
    }, { timeout: 3000 }).toBe(true);
    const metrics = await measureTarget();
    expect(metrics.cardWidth, `card width at ${width}px`).toBeGreaterThanOrEqual(44);
    expect(metrics.cardHeight, `card height at ${width}px`).toBeGreaterThanOrEqual(44);
    expect(metrics.titleHeight, `rendered title height at ${width}px`).toBeGreaterThanOrEqual(9);
    expect(Math.abs(metrics.centerX - metrics.graphCenterX), `horizontal centering at ${width}px`).toBeLessThanOrEqual(28);
    expect(Math.abs(metrics.centerY - metrics.graphCenterY), `vertical centering at ${width}px`).toBeLessThanOrEqual(28);
  }
  expect(seen.requests).toHaveLength(requestCount);
});

test('minimap tracks pan and zoom and recenters the map without an API request', async ({ page }) => {
  const seen = await observeMap(page);
  const minimap = page.locator('#tl-minimap');
  const viewport = page.locator('#tl-minimap-viewport');
  await expect(minimap).toBeVisible();
  await expect(viewport).toBeVisible();
  const requestCount = seen.requests.length;
  const initial = await viewport.boundingBox();

  await page.locator('#tl-zoom-in').click();
  await expect.poll(async () => {
    const zoomed = await viewport.boundingBox();
    return zoomed.width < initial.width && zoomed.height < initial.height;
  }).toBe(true);
  await page.locator('#tl-fit').click();
  await expect.poll(async () => {
    const fitted = await viewport.boundingBox();
    return Math.abs(fitted.x - initial.x) + Math.abs(fitted.y - initial.y)
      + Math.abs(fitted.width - initial.width) + Math.abs(fitted.height - initial.height);
  }).toBeLessThanOrEqual(2);

  const graph = page.locator('#tl-graph');
  const graphBox = await graph.boundingBox();
  const panStart = { x: graphBox.x + graphBox.width - 16, y: graphBox.y + 16 };
  const panTarget = await page.evaluate(({ x, y }) => {
    const target = document.elementFromPoint(x, y);
    return { tag: target?.tagName, blocked: Boolean(target?.closest('.tl-node,.tl-edge,#tl-minimap')) };
  }, panStart);
  expect(panTarget.blocked).toBe(false);
  await page.mouse.move(panStart.x, panStart.y);
  await page.mouse.down();
  await page.mouse.move(panStart.x - 80, panStart.y + 45);
  await page.mouse.up();
  await expect.poll(async () => {
    const panned = await viewport.boundingBox();
    return Math.abs(panned.x - initial.x) + Math.abs(panned.y - initial.y) > 10;
  }).toBe(true);
  const panned = await viewport.boundingBox();

  await viewport.focus();
  await page.keyboard.press('ArrowRight');
  await expect.poll(async () => viewport.boundingBox()).not.toEqual(panned);
  const keyboardCentered = await viewport.boundingBox();

  const miniBox = await minimap.boundingBox();
  await page.mouse.click(miniBox.x + miniBox.width * 0.8, miniBox.y + miniBox.height * 0.25);
  await expect.poll(async () => {
    const after = await viewport.boundingBox();
    return Math.abs(after.x - keyboardCentered.x) + Math.abs(after.y - keyboardCentered.y);
  }).toBeGreaterThan(10);

  const clicked = await viewport.boundingBox();
  await viewport.focus();
  await page.keyboard.press('Home');
  await expect.poll(async () => viewport.boundingBox()).not.toEqual(clicked);
  expect(seen.requests).toHaveLength(requestCount);

  await updateMap(page, seen, async () => {
    await page.locator('#tl-vrf').fill('missing-synthetic-context');
    await page.locator('#tl-vrf').press('Tab');
  }, (p) => p.filters.vrf === 'missing-synthetic-context');
  await expect(minimap).toBeHidden();
});

test('selection focus is disabled during a pending projection response', async ({ page }) => {
  const seen = await observeMap(page);
  let release;
  const blocked = new Promise((resolve) => { release = resolve; });
  let captured;
  const capturedResponse = new Promise((resolve) => { captured = resolve; });
  const handler = async (route) => {
    seen.requests.push(route.request().postDataJSON());
    const response = await route.fetch();
    captured(response);
    await blocked;
    await route.fulfill({ response });
  };
  await page.route('**/api/topology-layers', handler);
  try {
    await page.locator('#tl-vrf').fill('lab-vrf-01');
    await page.locator('#tl-vrf').press('Tab');
    await capturedResponse;
    await expect(page.locator('#tl-inspector')).toHaveAttribute('aria-busy', 'true');
    await expect(page.locator('#tl-focus-selection')).toBeDisabled();
    await expect(page.locator('#tl-minimap')).toBeHidden();
  } finally {
    release();
    await page.unroute('**/api/topology-layers', handler);
  }
  await expect(page.locator('#tl-inspector')).toHaveAttribute('aria-busy', 'false');
  await expect(page.locator('#tl-minimap')).toBeVisible();
  expect(seen.requests.length).toBeGreaterThan(1);
});
