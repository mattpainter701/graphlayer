const { test, expect } = require('@playwright/test');

test.use({ baseURL: 'http://127.0.0.1:8089' });

async function importBranchSample(page) {
  await page.goto('/#network-path');
  await page.locator('#network-load-exploration-sample').click();
  await expect(page.locator('#network-status')).toContainText(/4 sample firewall snapshots imported/i);
  await expect(page.locator('.network-node-row')).toHaveCount(4);
  await expect(page.locator('.network-link-row')).toHaveCount(4);
}

async function exploreWithRealWorker(page) {
  let saved;
  await page.route('**/api/network-explorations', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    saved = await response.json();
    return route.fulfill({ response });
  });
  await page.locator('#network-explore').click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  expect(saved.exploration.result.branches).toHaveLength(2);
  return saved.exploration;
}

async function exportAnalysis(page) {
  const [download] = await Promise.all([page.waitForEvent('download'), page.locator('#exploration-export').click()]);
  const stream = await download.createReadStream(); const chunks = [];
  for await (const chunk of stream) chunks.push(chunk);
  return JSON.parse(Buffer.concat(chunks).toString('utf8'));
}

async function boxesDoNotOverlap(page) {
  return page.locator('#exploration-topology').evaluate((svg) => {
    const matrixBox = (element) => {
      const box = element.getBBox();
      const matrix = element.getCTM();
      const points = [
        [box.x, box.y], [box.x + box.width, box.y],
        [box.x, box.y + box.height], [box.x + box.width, box.y + box.height],
      ].map(([x, y]) => {
        const point = svg.createSVGPoint(); point.x = x; point.y = y;
        const transformed = point.matrixTransform(matrix);
        return [transformed.x, transformed.y];
      });
      return {
        left: Math.min(...points.map((point) => point[0])),
        right: Math.max(...points.map((point) => point[0])),
        top: Math.min(...points.map((point) => point[1])),
        bottom: Math.max(...points.map((point) => point[1])),
      };
    };
    const intersects = (a, b) => a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;
    const nodes = [...svg.querySelectorAll('.topology-node')].map((group) => ({
      id: group.dataset.nodeId,
      card: group.querySelector('.topology-node-card'),
      icon: group.querySelector('.topology-device-icon'),
      label: group.querySelector('.topology-node-label'),
    }));
    const collisions = [];
    for (let i = 0; i < nodes.length; i++) for (let j = i + 1; j < nodes.length; j++) {
      for (const [nameA, elementA] of [['card', nodes[i].card], ['icon', nodes[i].icon], ['label', nodes[i].label]]) {
        for (const [nameB, elementB] of [['card', nodes[j].card], ['icon', nodes[j].icon], ['label', nodes[j].label]]) {
          if (elementA && elementB && intersects(matrixBox(elementA), matrixBox(elementB))) collisions.push(`${nodes[i].id} ${nameA} intersects ${nodes[j].id} ${nameB}`);
        }
      }
    }
    return { collisions, roles: nodes.map((entry) => entry.icon && entry.icon.dataset.icon), labels: nodes.map((entry) => entry.label && entry.label.textContent) };
  });
}

test('advanced topology filters and inspectors preserve real analysis, evidence ownership, and replay', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await importBranchSample(page);
  const exploration = await exploreWithRealWorker(page);
  const result = exploration.result;
  const branches = result.branches;
  const asymmetric = branches.flatMap((branch, branchIndex) => branch.returns.map((candidate, returnIndex) => ({ branchIndex, returnIndex, candidate }))).find((entry) => entry.candidate.asymmetric);
  const symmetric = branches.flatMap((branch, branchIndex) => branch.returns.map((candidate, returnIndex) => ({ branchIndex, returnIndex, candidate }))).find((entry) => !entry.candidate.asymmetric);
  expect(asymmetric).toBeTruthy();
  expect(symmetric).toBeTruthy();

  const canvas = page.locator('#exploration-topology');
  const compactBranch = page.locator('#topology-branch-select');
  const compactReturn = page.locator('#topology-return-select');
  await expect(compactBranch).toBeVisible();
  await expect(compactReturn).toBeVisible();
  const branchOptions = (locator) => locator.locator('option').evaluateAll((items) => items.map((item) => [item.value, item.textContent.trim()]));
  expect(await branchOptions(compactBranch)).toEqual(await branchOptions(page.locator('#exploration-branch')));
  expect(await branchOptions(compactReturn)).toEqual(await branchOptions(page.locator('#exploration-return')));
  await expect(canvas.locator('.topology-node')).toHaveCount(6);
  const initial = await canvas.locator('.topology-node').evaluateAll((nodes) => nodes.map((entry) => ({ id: entry.dataset.nodeId, role: entry.dataset.role, icon: entry.dataset.icon, source: entry.dataset.roleSource })));
  expect(initial.filter((item) => ['core', 'edge', 'north', 'south'].includes(item.id))).toHaveLength(4);
  expect(initial.filter((item) => item.role === 'firewall')).toHaveLength(4);
  for (const item of initial) {
    expect(item.role).toBeTruthy();
    expect(item.icon).toBeTruthy();
    expect(item.source).toBeTruthy();
  }
  const geometry = await boxesDoNotOverlap(page);
  expect(geometry.collisions).toEqual([]);
  expect(geometry.labels.every(Boolean)).toBe(true);

  await expect(page.locator('.topology-traffic-status[data-available="false"]')).toContainText(/traffic observations unavailable/i);
  const exportedBeforeFilters = await exportAnalysis(page);

  const filterNames = ['zone', 'subnet', 'vrf', 'vdom'];
  for (const name of filterNames) {
    const filter = page.locator(`.topology-filter[data-filter="${name}"]`);
    const facetValues = await canvas.locator('.topology-node').evaluateAll((nodes, facet) => [...new Set(nodes.flatMap((item) => (item.dataset[facet] || '').split(',').filter(Boolean)))].sort(), name);
    if (!facetValues.length) {
      await expect(filter).toHaveCount(0);
      continue;
    }
    await expect(filter).toHaveCount(1);
    const options = await filter.locator('option').evaluateAll((items) => items.map((item) => item.value).filter(Boolean).sort());
    expect(options).toEqual(facetValues);
    for (const value of facetValues) {
      await filter.selectOption(value);
      const visibility = await canvas.locator('.topology-node').evaluateAll((nodes, facetName) => nodes.map((item) => ({ id: item.dataset.nodeId, value: item.dataset[facetName], filtered: item.classList.contains('topology-filtered'), hidden: item.getAttribute('aria-hidden') === 'true' })), name);
      const expectedIDs = visibility.filter((item) => (item.value || '').split(',').includes(value)).map((item) => item.id).sort();
      expect(expectedIDs.length).toBeGreaterThan(0);
      expect(visibility.filter((item) => !item.filtered).map((item) => item.id).sort()).toEqual(expectedIDs);
      expect(visibility.filter((item) => item.filtered).every((item) => item.hidden)).toBe(true);
    }
    await filter.selectOption('');
    await expect(canvas.locator('.topology-node:not(.topology-filtered)')).toHaveCount(initial.length);
    await expect(page.locator('#exploration-disposition')).toContainText(result.disposition);
  }
  expect(await page.locator('#exploration-export').isEnabled()).toBe(true);
  expect(await exportAnalysis(page)).toEqual(exportedBeforeFilters);

  const links = canvas.locator('.topology-link[data-link-index]');
  await expect(links).toHaveCount(6);
  const declaredLinks = canvas.locator('.topology-link[data-provenance="configured_link"][data-synthetic="false"]');
  await expect(declaredLinks).toHaveCount(4);
  await declaredLinks.first().click();
  await expect(page.locator('#topology-inspector .topology-inspector-content[data-inspector-kind="link"]')).toBeVisible();
  await expect(page.locator('#topology-inspector')).toContainText(/configured_link/i);

  await compactBranch.selectOption(String(asymmetric.branchIndex));
  await expect(page.locator('#exploration-branch')).toHaveValue(String(asymmetric.branchIndex));
  await expect(page.locator('#exploration-return option')).toHaveCount(branches[asymmetric.branchIndex].returns.length);
  await expect(compactReturn.locator('option')).toHaveCount(branches[asymmetric.branchIndex].returns.length);
  await compactReturn.selectOption(String(asymmetric.returnIndex));
  await expect(page.locator('#exploration-return')).toHaveValue(String(asymmetric.returnIndex));
  const asymmetricForwardCount = branches[asymmetric.branchIndex].forward.hops.filter((hop) => hop.link).length;
  const asymmetricReturnCount = asymmetric.candidate.direction.hops.filter((hop) => hop.link).length;
  await expect(canvas.locator('.topology-edge.topology-forward')).toHaveCount(asymmetricForwardCount);
  await expect(canvas.locator('.topology-edge.topology-return')).toHaveCount(asymmetricReturnCount);
  await expect(page.locator('#exploration-return-status')).toContainText(/asymmetric route/i);
  await expect(page.locator('#topology-result-status')).toContainText(/asymmetric route/i);
  await compactBranch.selectOption(String(symmetric.branchIndex));
  await expect(page.locator('#exploration-branch')).toHaveValue(String(symmetric.branchIndex));
  await expect(page.locator('#exploration-return option')).toHaveCount(branches[symmetric.branchIndex].returns.length);
  await expect(compactReturn.locator('option')).toHaveCount(branches[symmetric.branchIndex].returns.length);
  await compactReturn.selectOption(String(symmetric.returnIndex));
  await expect(page.locator('#exploration-return')).toHaveValue(String(symmetric.returnIndex));
  await expect(canvas.locator('.topology-edge.topology-forward')).toHaveCount(branches[symmetric.branchIndex].forward.hops.filter((hop) => hop.link).length);
  await expect(canvas.locator('.topology-edge.topology-return')).toHaveCount(symmetric.candidate.direction.hops.filter((hop) => hop.link).length);
  await expect(page.locator('#exploration-return-status')).not.toContainText(/asymmetric route/i);

  const inspectorNode = 'edge';
  const projectedNode = initial.find((item) => item.id === inspectorNode);
  await canvas.locator(`.topology-node[data-node-id="${inspectorNode}"]`).click();
  const groups = page.locator('#topology-inspector details.topology-inspector-group');
  await expect(groups).toHaveCount(4);
  for (const group of ['topology', 'configuration', 'path', 'evidence']) {
    const section = page.locator(`#topology-inspector details[data-group="${group}"]`);
    await expect(section).toBeVisible();
    await expect(section).not.toHaveAttribute('open', '');
    await section.locator('summary').click();
    await expect(section).toHaveAttribute('open', '');
  }
  const rolePicker = page.locator('#topology-inspector .topology-role-select');
  for (const role of ['firewall', 'router', 'switch', 'host', 'server', 'network']) {
    await rolePicker.selectOption(role);
    await expect(canvas.locator(`.topology-node[data-node-id="${inspectorNode}"]`)).toHaveAttribute('data-role', role);
    await expect(canvas.locator(`.topology-node[data-node-id="${inspectorNode}"]`)).toHaveAttribute('data-icon', role);
    await expect(canvas.locator(`.topology-node[data-node-id="${inspectorNode}"]`)).toHaveAttribute('data-role-source', 'display_annotation');
  }
  await rolePicker.selectOption('');
  await expect(canvas.locator(`.topology-node[data-node-id="${inspectorNode}"]`)).toHaveAttribute('data-role', projectedNode.role);
  await expect(canvas.locator(`.topology-node[data-node-id="${inspectorNode}"]`)).toHaveAttribute('data-role-source', projectedNode.source);
  expect(await exportAnalysis(page)).toEqual(exportedBeforeFilters);
  for (const group of ['topology', 'configuration', 'path', 'evidence']) {
    const section = page.locator(`#topology-inspector details[data-group="${group}"]`);
    await section.locator('summary').click();
  }
  const evidence = page.locator('#topology-inspector .topology-source-button').first();
  await expect(evidence).toBeVisible();
  const evidenceRef = await evidence.evaluate((button) => ({ snapshot: button.dataset.snapshotId, ref: button.dataset.evidenceRef }));
  expect(evidenceRef.snapshot).toMatch(/^[a-f0-9]{64}$/);
  expect(evidenceRef.ref).toBeTruthy();
  const owner = await page.evaluate(async (id) => (await fetch(`/api/snapshots/${encodeURIComponent(id)}`)).json(), evidenceRef.snapshot);
  expect(owner.snapshot.hostname.toLowerCase()).toContain(inspectorNode);
  const sourceEntry = owner.snapshot.evidence.find((entry) => entry.id === evidenceRef.ref);
  expect(sourceEntry).toBeTruthy();
  const sourceLines = owner.snapshot.source_lines.filter((line) => line.number >= sourceEntry.start_line && line.number <= sourceEntry.end_line).map((line) => line.text).filter((line) => line.trim());
  expect(sourceLines.length).toBeGreaterThan(0);
  await evidence.click();
  await expect(page.locator('#source-lines')).toBeVisible();
  await expect(page.locator('#source-caption')).toContainText(owner.snapshot.hostname);
  for (const line of sourceLines) await expect(page.locator('#source-text')).toContainText(line);
  await page.locator('#close-source').click();

  await page.locator('#topology-fit').click();
  const fitted = await canvas.getAttribute('viewBox');
  await page.locator('#topology-zoom-in').click();
  await expect(page.locator('#topology-zoom')).toHaveText('120%');
  await page.locator('#topology-fit').click();
  await expect.poll(() => canvas.getAttribute('viewBox')).toBe(fitted);
  const stage = page.locator('#topology-stage');
  await stage.scrollIntoViewIfNeeded();
  const stageBox = await stage.boundingBox();
  const panPoint = await stage.evaluate((element) => {
    const box = element.getBoundingClientRect();
    const candidates = [[12, 12], [box.width - 12, 12], [12, box.height - 12], [box.width - 12, box.height - 12]];
    for (const [x, y] of candidates) {
      const hit = document.elementFromPoint(box.left + x, box.top + y);
      if (hit && hit.closest('#topology-stage') === element && !hit.closest('.topology-node,.topology-link')) return { x: box.left + x, y: box.top + y };
    }
    return null;
  });
  expect(panPoint).toBeTruthy();
  await page.mouse.move(panPoint.x, panPoint.y);
  await page.mouse.down(); await page.mouse.move(panPoint.x + 28, panPoint.y + 14); await page.mouse.up();
  await expect.poll(() => canvas.getAttribute('viewBox')).not.toBe(fitted);

  await page.reload();
  await page.goto('/#network-path');
  await expect(page.locator('#exploration-history-list .exploration-history-open').first()).toBeVisible();
  await page.locator('#exploration-history-list .exploration-history-open').first().click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  expect(await exportAnalysis(page)).toEqual(exploration);

  await page.setViewportSize({ width: 390, height: 844 });
  const viewport = await page.evaluate(() => ({ width: innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  expect(viewport.document).toBeLessThanOrEqual(390);
  expect(viewport.body).toBeLessThanOrEqual(390);
  expect((await boxesDoNotOverlap(page)).collisions).toEqual([]);
});
