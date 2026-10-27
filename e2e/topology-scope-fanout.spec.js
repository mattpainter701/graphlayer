const { test, expect } = require('@playwright/test');

async function observe(page) {
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
  return seen;
}

async function load(page, seen) {
  await page.goto('/#network-map');
  await page.locator('#tl-load').click();
  await expect.poll(() => seen.projections.length).toBeGreaterThan(0);
  await expect(page.locator('#tl-svg .tl-node').first()).toBeVisible();
  expect(seen.dataset.synthetic).toBe(true);
  expect(seen.dataset.hosts).toHaveLength(1000);
  expect(seen.dataset.networks).toHaveLength(10);
  return seen.dataset;
}

async function update(page, seen, action, predicate) {
  const before = seen.projections.length;
  await action();
  await expect.poll(() => seen.projections.slice(before).some(predicate), { timeout: 10_000 }).toBe(true);
  await expect(page.locator('#tl-inspector')).toHaveAttribute('aria-busy', 'false');
  return seen.projections.at(-1);
}

async function filter(page, seen, name, value) {
  return update(page, seen, async () => {
    await page.locator(`#tl-${name}`).fill(value);
    await page.locator(`#tl-${name}`).press('Tab');
  }, (p) => p.filters && p.filters[name === 'network' ? 'network_id' : name] === value);
}

function matching(dataset, filters) {
  return dataset.hosts.filter((h) => Object.entries(filters).every(([key, value]) => !value || h[key] === value));
}

async function assertSourceOwnership(page, dataset) {
  await expect(page.locator('#tl-sources')).toHaveAttribute('open', '');
  const shown = await page.locator('#tl-source-body').textContent();
  expect(dataset.sources.some((source) => shown.includes(source.sha256))).toBe(true);
  const cards = page.locator('#tl-source-body .tl-source-card');
  for (let index = 0; index < await cards.count(); index += 1) {
    const text = await cards.nth(index).textContent();
    const owner = dataset.sources.find((source) => text.includes(source.sha256));
    expect(owner).toBeTruthy();
    const excerpt = await cards.nth(index).locator('pre').textContent();
    for (const line of excerpt.split('\n').filter(Boolean)) {
      const [number, ...rest] = line.split(': ');
      expect(owner.lines.find((item) => item.number === Number(number)).text).toBe(rest.join(': '));
    }
  }
}

test('VRF and VDOM filters compose with network and zone and recover from an empty scope', async ({ page }) => {
  const seen = await observe(page);
  const dataset = await load(page, seen);
  const first = dataset.hosts[0];
  const vrfs = [...new Set(dataset.hosts.map((h) => h.vrf).filter(Boolean))].sort();
  const vdoms = [...new Set(dataset.hosts.map((h) => h.vdom).filter(Boolean))].sort();
  expect(vrfs.length).toBeGreaterThan(1);
  expect(vdoms.length).toBeGreaterThan(1);
  expect(seen.projections.at(-1).facets).toMatchObject({ vrfs, vdoms });
  await expect(page.locator('#tl-vrf-options option')).toHaveCount(vrfs.length);
  await expect(page.locator('#tl-vdom-options option')).toHaveCount(vdoms.length);

  await filter(page, seen, 'vrf', first.vrf);
  await filter(page, seen, 'vdom', first.vdom);
  await filter(page, seen, 'zone', first.zone);
  const scoped = await filter(page, seen, 'network', first.network_id);
  expect(scoped.pagination.total).toBe(matching(dataset, {
    vrf: first.vrf, vdom: first.vdom, zone: first.zone, network_id: first.network_id,
  }).length);
  expect(scoped.nodes.every((n) => n.vrf === first.vrf && n.vdom === first.vdom)).toBe(true);
  await expect(page.locator('[data-filter-kind="vrf"]')).toContainText(first.vrf);
  await expect(page.locator('[data-filter-kind="vdom"]')).toContainText(first.vdom);
  expect(scoped.facets).toMatchObject({ vrfs, vdoms });

  await update(page, seen, () => page.locator('[data-filter-kind="network"]').click(),
    (p) => !p.filters.network_id && p.filters.vrf === first.vrf);
  await update(page, seen, () => page.locator('[data-filter-kind="zone"]').click(),
    (p) => !p.filters.zone && p.filters.vdom === first.vdom);
  const zones = await update(page, seen, () => page.locator('#tl-tab-zone').click(), (p) => p.view === 'zone');
  const scopedHosts = matching(dataset, { vrf: first.vrf, vdom: first.vdom });
  const networkIds = new Set(scopedHosts.map((host) => host.network_id));
  const scopeNodes = [...dataset.networks, ...dataset.devices, ...dataset.hosts].filter((node) => networkIds.has(node.network_id || node.id));
  const allowedZones = new Set(scopeNodes.map((node) => node.zone));
  const allowedSources = new Set(scopeNodes.flatMap((node) => node.evidence).map((ref) => ref.source_id));
  expect(zones.nodes.every((node) => node.role === 'zone' && allowedZones.has(node.zone))).toBe(true);
  expect(zones.nodes.reduce((sum, node) => sum + node.host_count, 0)).toBe(scopedHosts.length);
  const zoneIds = new Set(zones.nodes.map((node) => node.id));
  expect(zones.links.every((link) => zoneIds.has(link.from) && zoneIds.has(link.to))).toBe(true);
  for (const ref of [...zones.nodes, ...zones.links].flatMap((item) => item.evidence)) {
    expect(allowedSources.has(ref.source_id)).toBe(true);
    const owner = dataset.sources.find((source) => source.id === ref.source_id);
    expect(owner.sha256).toBe(ref.sha256);
    expect(owner.lines.some((line) => line.number === ref.line)).toBe(true);
  }
  await update(page, seen, () => page.locator('#tl-svg').getByRole('button', { name: `zone: ${first.zone}`, exact: true }).click(),
    (p) => p.view === 'l3' && p.filters.vrf === first.vrf && p.filters.vdom === first.vdom);
  await expect(page.locator('#tl-vrf')).toHaveValue(first.vrf);
  await expect(page.locator('#tl-vdom')).toHaveValue(first.vdom);

  const empty = await filter(page, seen, 'vdom', 'missing-synthetic-context');
  expect(empty.nodes).toHaveLength(0);
  expect(empty.pagination.total).toBe(0);
  await expect(page.locator('#tl-caption')).toContainText(/no .*match|no elements/i);
  await update(page, seen, () => page.locator('[data-filter-kind="vdom"]').click(),
    (p) => p.filters.vdom === '' && p.filters.vrf === first.vrf && p.nodes.length > 0);
  await expect(page.locator('#tl-vrf')).toHaveValue(first.vrf);
  await update(page, seen, () => page.locator('#tl-reset-filters').click(),
    (p) => !p.filters.vrf && !p.filters.vdom && !p.filters.network_id && !p.filters.zone && p.pagination.total === 1000);
  await expect(page.locator('#tl-active-filters [data-filter-kind]')).toHaveCount(0);
});

test('100-host fan-out retains port and source drilldowns through a real scoped L2 trace', async ({ page }) => {
  const seen = await observe(page);
  const dataset = await load(page, seen);
  const src = dataset.hosts[0];
  const dst = dataset.hosts.find((h) => h.id === 'host-0075');
  await filter(page, seen, 'vrf', src.vrf);
  await filter(page, seen, 'vdom', src.vdom);
  await filter(page, seen, 'network', src.network_id);
  await update(page, seen, () => page.locator('#tl-tab-l2').click(), (p) => p.view === 'l2');
  const fanout = await update(page, seen, () => page.locator('#tl-page-size').selectOption('100'),
    (p) => p.pagination.limit === 100 && p.nodes.filter((n) => n.role === 'host').length === 100);
  expect(fanout.nodes.filter((n) => n.role === 'host')).toHaveLength(100);
  expect(fanout.nodes.every((n) => n.vrf === src.vrf && n.vdom === src.vdom)).toBe(true);
  const overlaps = await page.locator('#tl-svg').evaluate((svg) => {
    const boxes = [...svg.querySelectorAll('.tl-node')].map((node) => {
      const box = node.getBBox();
      const matrix = node.getCTM();
      const corners = [[box.x, box.y], [box.x + box.width, box.y + box.height]].map(([x, y]) => {
        const p = svg.createSVGPoint(); p.x = x; p.y = y; return p.matrixTransform(matrix);
      });
      return { l: corners[0].x, t: corners[0].y, r: corners[1].x, b: corners[1].y };
    });
    return boxes.flatMap((a, i) => boxes.slice(i + 1).filter((b) => a.l < b.r && a.r > b.l && a.t < b.b && a.b > b.t));
  });
  expect(overlaps).toEqual([]);
  expect(await page.locator('#tl-svg .tl-node').first().evaluate((node) => node.getBoundingClientRect().height)).toBeGreaterThan(8);
  await page.locator('#tl-svg').getByRole('button', { name: `host: ${src.id}`, exact: true }).click();
  await expect(page.locator('#tl-inspector')).toContainText(src.vrf);
  await expect(page.locator('#tl-inspector')).toContainText(src.vdom);
  await expect(page.locator('#tl-inspector')).toContainText(src.port);
  await page.locator('#tl-inspector').getByRole('button', { name: 'Use as source', exact: true }).click();
  await page.locator('#tl-inspector .tl-link-detail').first().click();
  await expect(page.locator('#tl-svg .tl-node[aria-pressed="true"]')).toHaveCount(0);
  await expect(page.locator('#tl-svg .tl-selected-edge[aria-pressed="true"]')).toHaveCount(1);
  const attachment = dataset.links.find((link) => link.from === src.id);
  await expect(page.locator('#tl-inspector')).toContainText(attachment.from_port);
  await expect(page.locator('#tl-inspector')).toContainText(attachment.to_port);
  await page.locator('#tl-evidence-button').click();
  await assertSourceOwnership(page, dataset);

  await page.locator('#tl-svg').getByRole('button', { name: `host: ${dst.id}`, exact: true }).click();
  await page.locator('#tl-inspector').getByRole('button', { name: 'Use as destination', exact: true }).click();
  const traced = await update(page, seen, () => page.locator('#tl-trace-button').click(),
    (p) => p.path && p.path.hops.length > 0);
  expect(seen.requests.at(-1)).toMatchObject({ source_host_id: src.id, destination_host_id: dst.id, vrf: src.vrf, vdom: src.vdom });
  expect(traced.path.hops[0].node_id || traced.path.hops[0].id).toBe(src.id);
  const last = traced.path.hops.at(-1);
  expect(last.node_id || last.id).toBe(dst.id);
  const posts = seen.requests.length;
  for (const width of [375, 768, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    await expect.poll(() => page.locator('#tl-svg .tl-node-box').evaluateAll((cards) => cards.every((card) => card.getBoundingClientRect().height >= 36))).toBe(true);
    await expect.poll(() => page.locator('#tl-svg .tl-node-title').evaluateAll((labels) => labels.every((label) => label.getBoundingClientRect().height >= 8))).toBe(true);
  }
  await page.setViewportSize({ width: 1600, height: 1000 });
  await page.locator('#tl-inspector-toggle').click();
  await expect.poll(() => page.locator('#tl-svg').evaluate((svg) => svg.viewBox.baseVal.width > svg.viewBox.baseVal.height)).toBe(true);
  await page.locator('#tl-inspector-toggle').click();
  // Inspector stacking can leave a wide canvas at 1280px; exercise the narrow layout explicitly.
  await page.setViewportSize({ width: 768, height: 900 });
  await expect.poll(() => page.locator('#tl-svg').evaluate((svg) => svg.viewBox.baseVal.height > svg.viewBox.baseVal.width)).toBe(true);
  const linksAtSwitch = page.locator('.tl-hop-list > li').nth(1).locator('.tl-hop-link');
  await expect(linksAtSwitch).toHaveCount(2);
  const linkLabels = await linksAtSwitch.allTextContents();
  expect(new Set(linkLabels).size).toBe(2);
  expect(linkLabels.join(' ')).toMatch(/ingress/i);
  expect(linkLabels.join(' ')).toMatch(/egress/i);
  const focusedHeight = await page.locator('#tl-svg').evaluate((svg) => svg.viewBox.baseVal.height);
  await page.locator('#tl-focus-path').click();
  const completeHeight = await page.locator('#tl-svg').evaluate((svg) => svg.viewBox.baseVal.height);
  expect(completeHeight).toBeGreaterThan(focusedHeight);
  await page.locator('#tl-focus-path').click();
  expect(await page.locator('#tl-svg').evaluate((svg) => svg.viewBox.baseVal.height)).toBe(focusedHeight);
  await page.locator('.tl-hop-select').nth(1).click();
  await expect(page.locator('#tl-inspector')).toContainText(traced.path.hops[1].node_id || traced.path.hops[1].id);
  await page.locator('.tl-hop-link').first().click();
  await expect(page.locator('#tl-inspector')).toContainText(/port|eth0|Gi1\/0/i);
  expect(seen.requests).toHaveLength(posts);
});

test('a trace outside the selected context fails closed without leaking path nodes', async ({ page }) => {
  const seen = await observe(page);
  const dataset = await load(page, seen);
  const selected = dataset.hosts[0];
  const outside = dataset.hosts.find((host) => host.vrf !== selected.vrf);
  expect(outside).toBeTruthy();
  await filter(page, seen, 'vrf', selected.vrf);
  await update(page, seen, () => page.locator('#tl-tab-l2').click(), (p) => p.view === 'l2');
  await page.locator('#tl-source').fill(outside.id);
  await page.locator('#tl-destination').fill(selected.id);
  const result = await update(page, seen, () => page.locator('#tl-trace-button').click(),
    (p) => p.path && p.path.reason === 'outside_selected_context');
  expect(result.path.hops).toEqual([]);
  expect(result.path.links).toEqual([]);
  expect(result.path.evidence).toEqual([]);
  expect(result.nodes.some((node) => node.id === outside.id)).toBe(false);
  await expect(page.locator('#tl-trace-status')).toContainText(/outside.*selected|selected.*context/i);
  await expect(page.locator('.tl-on-path')).toHaveCount(0);
});

test('scope filters and port details stay usable at mobile and tablet widths', async ({ page }) => {
  const seen = await observe(page);
  await load(page, seen);
  for (const width of [375, 768, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    for (const id of ['tl-vrf', 'tl-vdom', 'tl-network', 'tl-zone']) {
      await page.locator(`#${id}`).scrollIntoViewIfNeeded();
      const box = await page.locator(`#${id}`).boundingBox();
      expect(box.width).toBeGreaterThanOrEqual(120);
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width).toBeLessThanOrEqual(width + 1);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth + 1)).toBe(true);
  }
});
