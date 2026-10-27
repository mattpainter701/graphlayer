const { test, expect } = require('@playwright/test');

test.use({ baseURL: 'http://127.0.0.1:8089' });

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
  await page.goto('http://127.0.0.1:8088/#network-map');
  await page.locator('#tl-load').click();
  await expect.poll(() => seen.projections.length).toBeGreaterThan(0);
  expect(seen.dataset.synthetic).toBe(true);
  expect(seen.dataset.hosts).toHaveLength(1000);
  return seen;
}

async function updateMap(page, seen, action, predicate) {
  const prior = seen.projections.length;
  await action();
  await expect.poll(() => seen.projections.slice(prior).some(predicate), { timeout: 10_000 }).toBe(true);
  await expect(page.locator('#tl-inspector')).toHaveAttribute('aria-busy', 'false');
  return seen.projections.at(-1);
}

function hostRow(page, id) {
  return page.locator(`.tl-host-result[data-host-id="${id}"]`);
}

async function expectMapNodeCentered(page, id) {
  const metrics = () => page.locator('#tl-stage .tl-node').evaluateAll((nodes, nodeId) => {
    const node = nodes.find((item) => item.__item.id === nodeId);
    if (!node) return null;
    const card = node.querySelector('.tl-node-box').getBoundingClientRect();
    const graph = document.querySelector('#tl-graph').getBoundingClientRect();
    return {
      centered: Math.abs(card.left + card.width / 2 - (graph.left + graph.width / 2)) <= 28
        && Math.abs(card.top + card.height / 2 - (graph.top + graph.height / 2)) <= 28,
      id: node.__item.id,
    };
  }, id);
  await expect.poll(metrics).toMatchObject({ id, centered: true });
}

async function exploreSample(page, { omitNode, emptyReturn = false, holdBranchView = false, facetHideNode } = {}) {
  const seen = { analyses: [], views: [], holdNextView: false, waitForHeldView: null, releaseHeldView: null };
  let releaseView;
  let capturedView;
  const heldView = new Promise((resolve) => { releaseView = resolve; });
  const viewCaptured = new Promise((resolve) => { capturedView = resolve; });
  seen.waitForHeldView = () => viewCaptured;
  seen.releaseHeldView = () => releaseView();
  await page.route('**/api/network-explorations', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    const body = await response.json();
    if (emptyReturn && body.exploration?.result?.branches?.length) {
      const index = body.exploration.result.branches.findIndex((branch) => branch.forward?.hops?.length);
      if (index >= 0) {
        body.exploration.result.branches[index].returns = [];
        seen.emptyReturnBranch = index;
      }
    }
    seen.analyses.push(body.exploration);
    await route.fulfill({ response, json: body });
  });
  await page.route('**/api/topology-view', async (route) => {
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    const body = await response.json();
    seen.views.push(body.topology);
    if (omitNode && body.topology && Array.isArray(body.topology.nodes)) {
      body.topology.nodes = body.topology.nodes.filter((node) => node.node_id !== omitNode);
    }
    if (facetHideNode && body.topology && Array.isArray(body.topology.nodes)) {
      const target = body.topology.nodes.find((node) => node.node_id === facetHideNode);
      if (target) {
        target.vrf = `e2e-hidden-${facetHideNode}`;
        (target.interfaces || []).forEach((iface) => { iface.vrf = target.vrf; });
        const facet = body.topology.facets?.vrf;
        const values = Array.isArray(facet) ? facet : facet && facet.values;
        if (Array.isArray(values)) values.push(target.vrf);
      }
    }
    if (holdBranchView && seen.holdNextView && !seen.heldView) {
      seen.heldView = true;
      capturedView();
      await heldView;
    }
    await route.fulfill({ response, json: body });
  });

  await page.goto('/#network-path');
  await page.locator('#network-load-exploration-sample').click();
  await expect(page.locator('#network-status')).toContainText('4 sample firewall snapshots imported');
  await page.locator('#network-explore').click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  await expect.poll(() => seen.analyses.length).toBe(1);
  await expect.poll(() => seen.views.length).toBeGreaterThan(0);
  await expect(page.locator('#topology-path-hop-buttons')).toBeVisible();
  return seen;
}

function findBranch(exploration, predicate = () => true) {
  return exploration.result.branches.findIndex((branch) => predicate(branch));
}

async function selectBranch(page, index, returnIndex = 0) {
  await page.locator('#topology-branch-select').selectOption(String(index));
  await page.locator('#topology-return-select').selectOption(String(returnIndex));
  await expect(page.locator('#topology-path-hop-buttons .topology-path-hop-button')).not.toHaveCount(0);
}

test('path stepper changes direction, owns exact hop ports, and resets when its branch changes', async ({ page }) => {
  const seen = await exploreSample(page);
  const exploration = seen.analyses[0];
  const branchIndex = findBranch(exploration, (branch) => branch.forward.hops.length > 1
    && branch.forward.hops[1].path
    && (branch.forward.hops[1].path.ingress_interface || branch.forward.hops[1].path.ingress || branch.forward.hops[1].path.egress_interface));
  expect(branchIndex).toBeGreaterThanOrEqual(0);
  await selectBranch(page, branchIndex);

  const forwardButtons = page.locator('#topology-path-hop-buttons .topology-path-hop-button');
  await expect(forwardButtons.first()).toHaveAttribute('aria-current', 'step');
  await page.locator('#topology-path-next').click();
  await expect(page.locator('#topology-path-status')).toContainText(/Forward hop 2 of/);
  await expect(forwardButtons.nth(1)).toHaveAttribute('aria-current', 'step');
  await expect(page.locator('#topology-path-next')).toBeFocused();
  const hop = exploration.result.branches[branchIndex].forward.hops[1];
  await expect(page.locator('#topology-inspector')).toContainText(hop.node_id);
  const portNames = [hop.path.ingress_interface || hop.path.ingress, hop.path.egress_interface].filter(Boolean);
  const uniquePorts = [...new Set(portNames)];
  const portButtons = page.locator('#topology-path-port-shortcuts .topology-path-port-button');
  await expect(portButtons).toHaveCount(uniquePorts.length);
  expect(await portButtons.evaluateAll((buttons) => buttons.map((button) => button.dataset.portName))).toEqual(uniquePorts);

  if (uniquePorts.length) {
    await portButtons.first().click();
    const portDetails = page.locator(`#topology-inspector .topology-port-detail[data-node-id="${hop.node_id}"][data-port-detail="${uniquePorts[0]}"]`);
    await expect(portDetails).toBeVisible();
    await expect(portDetails.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');
    expect(await portDetails.getAttribute('data-node-id')).toBe(hop.node_id);
    const snapshotId = exploration.request.nodes.find((item) => item.node_id === hop.node_id)?.snapshot_id;
    expect(snapshotId).toMatch(/^[a-f0-9]{64}$/);
    const sourceOwners = await portDetails.locator('.topology-source-button').evaluateAll((buttons) => buttons.map((button) => button.dataset.snapshotId));
    if (sourceOwners.length) expect(sourceOwners.every((owner) => owner === snapshotId)).toBe(true);
  }

  const returnDirection = page.locator('#topology-path-direction');
  await returnDirection.selectOption('return');
  await expect(page.locator('#topology-path-status')).toContainText(/Return hop 1 of/);
  await expect(page.locator('#topology-path-hop-buttons .topology-path-hop-button').first()).toHaveAttribute('aria-current', 'step');
  const secondBranch = exploration.result.branches.findIndex((branch, index) => index !== branchIndex && branch.forward.hops.length > 1);
  if (secondBranch >= 0) {
    await page.locator('#topology-branch-select').selectOption(String(secondBranch));
    await expect.poll(() => seen.views.length).toBeGreaterThan(1);
    await expect(page.locator('#topology-path-direction')).toHaveValue('forward');
    await expect(page.locator('#topology-path-status')).toContainText(/Forward hop 1 of/);
    await expect(page.locator('#topology-path-hop-buttons .topology-path-hop-button').first()).toHaveAttribute('aria-current', 'step');
  }
  expect(seen.analyses).toHaveLength(1);
  const viewsAfterPathNavigation = seen.views.length;
  await page.locator('#topology-path-next').click();
  expect(seen.analyses).toHaveLength(1);
  expect(seen.views).toHaveLength(viewsAfterPathNavigation);
});

test('path hop whose node is absent from the returned topology projection is disabled', async ({ page }) => {
  const seen = await exploreSample(page, { omitNode: 'north' });
  const exploration = seen.analyses[0];
  const branchIndex = findBranch(exploration, (branch) => branch.forward.hops.some((hop) => hop.node_id === 'north'));
  expect(branchIndex).toBeGreaterThanOrEqual(0);
  await selectBranch(page, branchIndex);
  const unavailable = page.locator('#topology-path-hop-buttons .topology-path-hop-button[aria-label*="north"]');
  await expect(unavailable).toBeDisabled();
  await expect(unavailable).toHaveAttribute('title', /not present in the current topology projection/i);
  const branch = exploration.result.branches[branchIndex];
  const missingIndex = branch.forward.hops.findIndex((hop) => hop.node_id === 'north');
  expect(missingIndex).toBeGreaterThan(0);
  const priorHop = branch.forward.hops[missingIndex - 1];
  const priorButton = page.locator(`#topology-path-hop-buttons .topology-path-hop-button[data-hop-index="${missingIndex - 1}"]`);
  await priorButton.click();
  await expect(page.locator('#topology-path-port-shortcuts .topology-path-port-button')).not.toHaveCount(0);
  await page.locator('#topology-path-next').click();
  await expect(page.locator('#topology-path-status')).toContainText(/north is unavailable in the current topology projection/i);
  await expect(page.locator('#topology-path-port-shortcuts .topology-path-port-button')).toHaveCount(0);
  expect(priorHop.node_id).not.toBe('north');
  expect(seen.analyses).toHaveLength(1);
});

test('empty return path can switch back to the available forward path', async ({ page }) => {
  const seen = await exploreSample(page, { emptyReturn: true });
  const branchIndex = seen.emptyReturnBranch;
  expect(branchIndex).toBeGreaterThanOrEqual(0);
  await page.locator('#topology-branch-select').selectOption(String(branchIndex));
  await expect(page.locator('#topology-path-status')).toContainText(/Forward hop 1 of/);
  const direction = page.locator('#topology-path-direction');
  await direction.selectOption('return');
  await expect(direction).toBeEnabled();
  await expect(page.locator('#topology-path-status')).toContainText(/No selected return path hops/i);
  await expect(page.locator('#topology-path-hop-buttons .topology-path-hop-button')).toHaveCount(0);
  await direction.selectOption('forward');
  await expect(page.locator('#topology-path-status')).toContainText(/Forward hop 1 of/);
  await expect(page.locator('#topology-path-hop-buttons .topology-path-hop-button').first()).toHaveAttribute('aria-current', 'step');
  expect(seen.analyses).toHaveLength(1);
});

test('changing branches hides and inerts the navigator until the new projection is ready', async ({ page }) => {
  const seen = await exploreSample(page, { holdBranchView: true });
  const exploration = seen.analyses[0];
  const first = findBranch(exploration, (branch) => branch.forward.hops.length > 1);
  const second = exploration.result.branches.findIndex((branch, index) => index !== first && branch.forward.hops.length > 1);
  expect(first).toBeGreaterThanOrEqual(0);
  expect(second).toBeGreaterThanOrEqual(0);
  await selectBranch(page, first);
  const portBearing = exploration.result.branches[first].forward.hops.findIndex((hop) => hop.path && (hop.path.ingress_interface || hop.path.ingress || hop.path.egress_interface));
  expect(portBearing).toBeGreaterThanOrEqual(0);
  await page.locator(`#topology-path-hop-buttons .topology-path-hop-button[data-hop-index="${portBearing}"]`).click();
  const shortcuts = page.locator('#topology-path-port-shortcuts .topology-path-port-button');
  await expect(shortcuts).not.toHaveCount(0);
  seen.holdNextView = true;
  await page.locator('#topology-branch-select').selectOption(String(second));
  await seen.waitForHeldView();
  const navigator = page.locator('#topology-path-navigator');
  await expect(navigator).toBeHidden();
  await expect(navigator).toHaveAttribute('inert', '');
  await expect(navigator).toHaveAttribute('aria-busy', 'true');
  await expect(shortcuts).toHaveCount(0);
  seen.releaseHeldView();
  await expect(navigator).toBeVisible();
  await expect(navigator).toHaveAttribute('aria-busy', 'false');
  await expect(page.locator('#topology-path-direction')).toHaveValue('forward');
  await expect(page.locator('#topology-path-status')).toContainText(/Forward hop 1 of/);
  expect(seen.analyses).toHaveLength(1);
});

test('hop navigation into a facet-hidden node preserves filter chips and navigation focus', async ({ page }) => {
  const seen = await exploreSample(page, { facetHideNode: 'north' });
  const exploration = seen.analyses[0];
  const branchIndex = findBranch(exploration, (branch) => branch.forward.hops.some((hop) => hop.node_id === 'north'));
  expect(branchIndex).toBeGreaterThanOrEqual(0);
  await selectBranch(page, branchIndex);
  const branch = exploration.result.branches[branchIndex];
  const hiddenIndex = branch.forward.hops.findIndex((hop) => hop.node_id === 'north');
  expect(hiddenIndex).toBeGreaterThan(0);
  await page.locator(`#topology-path-hop-buttons .topology-path-hop-button[data-hop-index="${hiddenIndex - 1}"]`).click();
  const activeFilter = page.locator('#topology-filters [data-filter="vrf"]');
  const filterValue = await activeFilter.locator('option').evaluateAll((options) => options.map((option) => option.value).find((value) => value && !value.startsWith('e2e-hidden-')));
  expect(filterValue).toBeTruthy();
  await activeFilter.selectOption(filterValue);
  const chips = await page.locator('#topology-active-filters .topology-filter-chip').allTextContents();
  expect(chips).toHaveLength(1);
  await page.locator('#topology-path-next').click();
  await expect(page.locator('#topology-path-status')).toContainText(/north is unavailable under the active filters/i);
  const filteredHop = page.locator(`#topology-path-hop-buttons .topology-path-hop-button[data-hop-index="${hiddenIndex}"]`);
  await expect(filteredHop).toHaveAttribute('aria-label', /north is unavailable under the active filters/i);
  await expect(filteredHop).toBeDisabled();
  await expect(page.locator('#topology-path-port-shortcuts .topology-path-port-button')).toHaveCount(0);
  await expect(page.locator('#topology-path-next')).toBeFocused();
  expect(await page.locator('#topology-active-filters .topology-filter-chip').allTextContents()).toEqual(chips);
  expect(seen.analyses).toHaveLength(1);
});

test('path controls remain reachable and the topology page has no horizontal overflow at common widths', async ({ page }) => {
  const seen = await exploreSample(page);
  const exploration = seen.analyses[0];
  const branchIndex = findBranch(exploration, (branch) => branch.forward.hops.length > 1);
  expect(branchIndex).toBeGreaterThanOrEqual(0);
  await selectBranch(page, branchIndex);
  for (const width of [375, 768, 1280]) {
    await page.setViewportSize({ width, height: 850 });
    const dimensions = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
    expect(dimensions.viewport).toBe(width);
    expect(dimensions.document, JSON.stringify(dimensions)).toBeLessThanOrEqual(width);
    expect(dimensions.body, JSON.stringify(dimensions)).toBeLessThanOrEqual(width);
    const button = page.locator('#topology-path-hop-buttons .topology-path-hop-button').nth(1);
    await button.focus();
    await expect(button).toBeFocused();
    const size = await button.boundingBox();
    expect(size.width).toBeGreaterThanOrEqual(width < 600 ? 44 : 40);
    expect(size.height).toBeGreaterThanOrEqual(width < 600 ? 44 : 40);
  }
  expect(seen.analyses).toHaveLength(1);
});

test('fullscreen keeps path controls compact and the canvas and inspector usable across widths', async ({ page }) => {
  const seen = await exploreSample(page);
  const exploration = seen.analyses[0];
  const branchIndex = findBranch(exploration, (branch) => branch.forward.hops.length === 3);
  expect(branchIndex).toBeGreaterThanOrEqual(0);
  await selectBranch(page, branchIndex);
  const hops = exploration.result.branches[branchIndex].forward.hops;
  const hopIndex = hops.findIndex((hop) => hop.path && (hop.path.ingress_interface || hop.path.ingress || hop.path.egress_interface));
  expect(hopIndex).toBeGreaterThanOrEqual(0);
  await page.locator(`#topology-path-hop-buttons .topology-path-hop-button[data-hop-index="${hopIndex}"]`).click();
  const navigator = page.locator('#topology-path-navigator');
  const controls = navigator.locator('.topology-path-hop-button, .topology-path-port-button');
  await expect(controls).not.toHaveCount(0);
  await expect(page.locator('#topology-inspector-toggle')).toHaveAttribute('aria-expanded', 'true');
  await expect(page.locator('#topology-inspector')).toBeVisible();
  await page.locator('#topology-fullscreen').click();
  const workbench = page.locator('.topology-workbench');
  await expect(workbench).toHaveClass(/topology-fullscreen-active/);

  for (const [width, height] of [[1280, 900], [768, 900], [375, 720]]) {
    await page.setViewportSize({ width, height });
    await navigator.scrollIntoViewIfNeeded();
    await expect(navigator).toBeVisible();
    const layout = await page.evaluate(() => {
      const root = document.querySelector('.topology-workbench');
      const nav = document.querySelector('#topology-path-navigator');
      const body = document.querySelector('.topology-workbench-body');
      const stage = document.querySelector('.topology-stage');
      const inspector = document.querySelector('#topology-inspector');
      const rect = (element) => { const value = element.getBoundingClientRect(); return { x: value.x, y: value.y, width: value.width, height: value.height, right: value.right, bottom: value.bottom }; };
      return {
        viewport: { width: innerWidth, height: innerHeight, documentWidth: document.documentElement.scrollWidth },
        root: { ...rect(root), scrollWidth: root.scrollWidth, clientWidth: root.clientWidth }, nav: rect(nav), body: rect(body), stage: rect(stage), inspector: rect(inspector),
        controls: [...nav.querySelectorAll('.topology-path-hop-button, .topology-path-port-button')].map((element) => ({ ...rect(element), text: element.textContent, scrollWidth: element.scrollWidth, clientWidth: element.clientWidth, scrollHeight: element.scrollHeight, clientHeight: element.clientHeight })),
      };
    });
    expect(layout.viewport.documentWidth).toBeLessThanOrEqual(width + 1);
    expect(layout.root.width).toBeLessThanOrEqual(width);
    expect(layout.root.scrollWidth).toBeLessThanOrEqual(layout.root.clientWidth + 1);
    expect(layout.nav.height, `navigator height at ${width}px`).toBeLessThan(width < 600 ? 400 : 300);
    expect(layout.body.height).toBeGreaterThanOrEqual(300);
    expect(layout.stage.height).toBeGreaterThanOrEqual(300);
    expect(layout.controls.length).toBeGreaterThan(0);
    for (const control of layout.controls) {
      expect(control.x).toBeGreaterThanOrEqual(layout.nav.x - 1);
      expect(control.right).toBeLessThanOrEqual(layout.nav.right + 1);
      expect(control.y).toBeGreaterThanOrEqual(layout.nav.y - 1);
      expect(control.bottom).toBeLessThanOrEqual(layout.nav.bottom + 1);
      expect(control.scrollWidth).toBeLessThanOrEqual(control.clientWidth + 1);
      expect(control.scrollHeight).toBeLessThanOrEqual(control.clientHeight + 1);
      expect(control.text.trim().length).toBeGreaterThan(0);
    }
    if (width <= 800) expect(layout.inspector.y).toBeGreaterThanOrEqual(layout.stage.bottom - 1);
    else expect(layout.inspector.x).toBeGreaterThanOrEqual(layout.stage.right - 1);
    await expect(controls.last()).toBeVisible();
  }

  await page.locator('#topology-fullscreen').click();
  await expect(workbench).not.toHaveClass(/topology-fullscreen-active/);
  await expect(navigator).toBeVisible();
  await navigator.scrollIntoViewIfNeeded();
  await page.locator('#topology-path-direction').focus();
  await expect(page.locator('#topology-path-direction')).toBeFocused();
  expect(seen.analyses).toHaveLength(1);
});

test('Show on map centers the exact returned host on a 100 host page and preserves VRF/VDOM scope', async ({ page }) => {
  const seen = await observeMap(page);
  await updateMap(page, seen, () => page.locator('#tl-tab-l2').click(), (projection) => projection.view === 'l2');
  await updateMap(page, seen, async () => {
    await page.locator('#tl-vrf').fill('lab-vrf-01');
    await page.locator('#tl-vdom').fill('campus');
    await page.locator('#tl-apply-filters').click();
  }, (projection) => projection.filters.vrf === 'lab-vrf-01' && projection.filters.vdom === 'campus');
  const page100 = await updateMap(page, seen, () => page.locator('#tl-page-size').selectOption('100'), (projection) => projection.pagination.limit === 100);
  expect(page100.hosts).toHaveLength(100);
  expect(page100.counts.hosts).toBe(1000);
  expect(page100.nodes.length).toBeLessThanOrEqual(140);
  expect(page100.filters).toMatchObject({ vrf: 'lab-vrf-01', vdom: 'campus' });
  const targetHost = seen.dataset.hosts.find((host) => host.vrf === 'lab-vrf-01' && host.vdom === 'campus');
  expect(targetHost).toBeTruthy();
  const priorCount = seen.projections.length;
  const mapped = await updateMap(page, seen, () => hostRow(page, targetHost.id).locator('[data-action="map"]').click(),
    (projection) => projection.navigation.focus_host_id === targetHost.id && projection.navigation.focus_status === 'visible');
  expect(seen.projections.length).toBe(priorCount + 1);
  expect(mapped.navigation).toMatchObject({ focus_host_id: targetHost.id, focus_status: 'visible' });
  expect(mapped.nodes.some((node) => node.id === targetHost.id)).toBe(true);
  expect(mapped.nodes.length).toBeLessThanOrEqual(140);
  expect(mapped.pagination).toMatchObject({ limit: 100 });
  expect(mapped.filters).toMatchObject({ vrf: 'lab-vrf-01', vdom: 'campus' });
  await expect(page.locator('#tl-vrf')).toHaveValue('lab-vrf-01');
  await expect(page.locator('#tl-vdom')).toHaveValue('campus');
  await expect.poll(() => page.evaluate(() => document.activeElement?.__item?.id || '')).toBe(targetHost.id);
  await expectMapNodeCentered(page, targetHost.id);
});

test('minimap node pointer hit selects and centers that exact returned node without another projection', async ({ page }) => {
  const seen = await observeMap(page);
  const l2 = await updateMap(page, seen, () => page.locator('#tl-tab-l2').click(), (projection) => projection.view === 'l2');
  expect(l2.nodes.length).toBeGreaterThan(0);
  await page.locator('#tl-zoom-in').click();
  await page.locator('#tl-minimap').scrollIntoViewIfNeeded();
  const requestCount = seen.requests.length;
  const hit = await page.locator('#tl-minimap [data-mini-key]').evaluateAll((nodes) => {
    const fractions = [[.5,.5],[.25,.5],[.75,.5],[.5,.25],[.5,.75]];
    for (const node of nodes) {
      const box = node.getBoundingClientRect();
      for (const [fx, fy] of fractions) {
        const x = box.left + box.width * fx;
        const y = box.top + box.height * fy;
        const top = document.elementFromPoint(x, y);
        if (top?.closest('[data-mini-key]') === node) return { x, y, key: node.dataset.miniKey };
      }
    }
    return null;
  });
  expect(hit, 'a real pointer coordinate must hit a minimap node rect rather than the viewport overlay').toBeTruthy();
  const targetId = hit.key.replace(/^node:/, '');
  expect(l2.nodes.some((node) => node.id === targetId)).toBe(true);
  await page.mouse.click(hit.x, hit.y);
  await expect.poll(() => page.evaluate(() => document.activeElement?.__item?.id || '')).toBe(targetId);
  await expectMapNodeCentered(page, targetId);
  expect(seen.requests).toHaveLength(requestCount);
});
