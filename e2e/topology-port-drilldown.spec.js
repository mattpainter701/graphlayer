const { test, expect } = require('@playwright/test');

test.use({ baseURL: 'http://127.0.0.1:8089' });

async function loadAndAnalyze(page) {
  let analyses = 0;
  let exploration = null;
  let submittedLinks = [];
  await page.route('**/api/network-explorations', async (route) => {
    if (route.request().method() !== 'POST') return route.continue();
    analyses += 1;
    const requestBody = route.request().postDataJSON();
    submittedLinks = requestBody.links || [];
    const response = await route.fetch();
    const body = await response.json();
    exploration = body.exploration;
    await route.fulfill({ response });
  });
  await page.goto('/#network-path');
  await page.locator('#network-load-exploration-sample').click();
  await expect(page.locator('#network-status')).toContainText(/4 sample firewall snapshots imported/i);
  await page.locator('#network-explore').click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  await expect.poll(() => analyses).toBe(1);
  return { exploration, submittedLinks, analyses: () => analyses };
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

async function openInspectorGroup(page, group) {
  const details = page.locator(`#topology-inspector details[data-group="${group}"]`);
  await expect(details).toBeVisible();
  if (!(await details.evaluate((item) => item.open))) await details.locator('summary').click();
  await expect(details).toHaveAttribute('open', '');
  return details;
}

test('inspects an analyzed edge port, its declared link, selected hops, and owned evidence', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const { exploration, submittedLinks } = await loadAndAnalyze(page);
  expect(exploration?.result?.branches).toHaveLength(2);
  await expect(page.locator('.app-page[data-app-page="topology"]')).toBeVisible();

  const edge = await snapshotForNode(page, 'edge');
  const interfacesUsedByHop = (hop) => [hop?.path?.ingress_interface || hop?.path?.ingress, hop?.path?.egress_interface].filter(Boolean);
  const links = exploration.request.links || submittedLinks;
  expect(links.length).toBeGreaterThan(0);
  const declaredPorts = new Set(links.flatMap((link) => [link.a, link.b].filter((endpoint) => endpoint.node_id === 'edge').map((endpoint) => endpoint.interface)));
  const configuredPorts = new Set(edge.path.interfaces.map((item) => item.name || item.physical));
  expect([...declaredPorts].some((name) => configuredPorts.has(name))).toBe(true);
  const selected = exploration.result.branches.flatMap((candidate, branchIndex) => candidate.returns.map((returned, returnIndex) => ({ candidate, branchIndex, returned, returnIndex }))).find(({ candidate, returned }) => {
    const forward = candidate.forward.hops.find((hop) => hop.node_id === 'edge');
    const reverse = returned.direction.hops.find((hop) => hop.node_id === 'edge');
    return Boolean(forward && reverse);
  });
  expect(selected).toBeTruthy();
  const { candidate: branch, branchIndex, returnIndex } = selected;
  await page.locator('#topology-branch-select').selectOption(String(branchIndex));
  await page.locator('#topology-return-select').selectOption(String(returnIndex));
  expect(branch).toBeTruthy();
  const edgeHop = branch.forward.hops.find((hop) => hop.node_id === 'edge');
  const returnHop = branch.returns[returnIndex].direction.hops.find((hop) => hop.node_id === 'edge');
  const edgePort = [...declaredPorts].find((name) => configuredPorts.has(name) && interfacesUsedByHop(edgeHop).includes(name));
  expect(edgePort).toBeTruthy();
  const projectedInterface = edge.path.interfaces.find((item) => (item.name || item.physical) === edgePort);
  expect(projectedInterface).toBeTruthy();
  const declaredLink = links.find((link) => (link.a.node_id === 'edge' && link.a.interface === edgePort) || (link.b.node_id === 'edge' && link.b.interface === edgePort));
  expect(declaredLink).toBeTruthy();
  expect(returnHop).toBeTruthy();
  const remote = declaredLink.a.node_id === 'edge' ? declaredLink.b : declaredLink.a;

  await page.locator('#topology-device-list .topology-device[data-node-id="edge"]').click();
  const pathGroup = await openInspectorGroup(page, 'path');
  await expect(pathGroup.locator('.topology-hop-card[data-hop-direction="forward"][data-hop-node-id="edge"]')).toBeVisible();
  await expect(pathGroup.locator('.topology-hop-card[data-hop-direction="return"][data-hop-node-id="edge"]')).toBeVisible();
  const portGroup = await openInspectorGroup(page, 'topology');
  const portDetails = portGroup.locator(`.topology-port-detail[data-port-detail="${edgePort}"]`);
  await expect(portDetails).toBeVisible();
  const portToggle = portDetails.locator('.topology-port-toggle');
  await portToggle.focus();
  await page.keyboard.press('Enter');
  await expect(portToggle).toHaveAttribute('aria-expanded', 'true');
  await expect(portDetails.locator('.topology-port-detail-body')).toBeVisible();
  await expect(portDetails).toContainText(edgePort);
  await expect(portDetails).toContainText(remote.node_id);
  await expect(portDetails).toContainText(projectedInterface.address || projectedInterface.ip_address || 'Unavailable');
  await expect(portDetails.locator('.topology-hop-card[data-hop-direction="forward"][data-hop-node-id="edge"]')).toBeVisible();

  await expect(portDetails).toContainText(/declared links for this port/i);
  await portDetails.locator('.topology-neighbor-link[data-link-index]').first().click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'link');
  const linkPort = page.locator(`#topology-inspector .topology-endpoint-button[data-interface="${edgePort}"]`);
  await expect(linkPort).toBeVisible();
  await linkPort.click();
  await expect(page.locator('#topology-inspector .topology-inspector-content')).toHaveAttribute('data-inspector-kind', 'device');
  await expect(page.locator('#topology-inspector details[data-group="topology"]')).toHaveAttribute('open', '');
  const jumpedPort = page.locator(`#topology-inspector .topology-port-detail[data-port-detail="${edgePort}"]`);
  await expect(jumpedPort.locator('.topology-port-toggle')).toHaveAttribute('aria-expanded', 'true');
  await expect(jumpedPort.locator('.topology-port-detail-body')).toBeVisible();

  const selectedEvidence = jumpedPort.locator('.topology-source-button').first();
  await expect(selectedEvidence).toBeVisible();
  const ref = await selectedEvidence.evaluate((button) => ({ snapshotId: button.dataset.snapshotId, evidenceRef: button.dataset.evidenceRef }));
  expect(ref.snapshotId).toBe(edge.id);
  const owned = edge.evidence.find((item) => item.id === ref.evidenceRef);
  expect(owned).toBeTruthy();
  expect(owned.start_line).toBeGreaterThan(0);
  expect(owned.end_line).toBeGreaterThanOrEqual(owned.start_line);
  const sourceLines = edge.source_lines.filter((line) => line.number >= owned.start_line && line.number <= owned.end_line);
  expect(sourceLines.length).toBeGreaterThan(0);
  await selectedEvidence.click();
  await expect(page.locator('#source-lines')).toBeVisible();
  await expect(page.locator('#source-caption')).toContainText(edge.hostname);
  await expect(page.locator('#source-caption')).toContainText(edge.id.slice(0, 12));
});

test('refreshes selected port hops with branch and return choices and keeps unavailable values clear on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  const { exploration, analyses } = await loadAndAnalyze(page);
  const branches = exploration.result.branches;
  expect(branches).toHaveLength(2);

  const branchChoices = branches.map((branch, index) => ({ branch, index, edge: branch.forward.hops.find((hop) => hop.node_id === 'edge') }));
  expect(branchChoices.every((choice) => choice.edge)).toBe(true);
  await page.locator('#topology-device-list .topology-device[data-node-id="edge"]').click();
  await openInspectorGroup(page, 'path');
  const selected = page.locator('#topology-inspector details[data-group="path"] .topology-hop-card[data-hop-direction="forward"][data-hop-node-id="edge"]');
  const firstNextHop = branchChoices[0].edge.path.selected_route?.next_hop;
  const secondNextHop = branchChoices[1].edge.path.selected_route?.next_hop;
  expect(firstNextHop).toBeTruthy();
  expect(secondNextHop).toBeTruthy();
  expect(secondNextHop).not.toBe(firstNextHop);
  await expect(selected).toContainText(firstNextHop);

  await page.locator('#topology-branch-select').selectOption(String(branchChoices[1].index));
  await expect(page.locator('#topology-inspector details[data-group="path"]')).toHaveAttribute('open', '');
  await expect.poll(() => page.locator('#topology-inspector details[data-group="path"] .topology-hop-card[data-hop-direction="forward"][data-hop-node-id="edge"]').innerText()).toContain(branchChoices[1].edge.path.selected_route?.next_hop || branchChoices[1].edge.path.egress_interface);
  const returnCandidate = branches[branchChoices[1].index].returns[0];
  expect(returnCandidate).toBeTruthy();
  await page.locator('#topology-return-select').selectOption('0');
  await expect(page.locator('#topology-inspector details[data-group="path"] .topology-hop-card[data-hop-direction="return"]')).toHaveCount(returnCandidate.direction.hops.filter((hop) => hop.node_id === 'edge').length);
  expect(analyses()).toBe(1);

  const south = await snapshotForNode(page, 'south');
  const southInterface = south.path.interfaces.find((item) => (item.name || item.physical) === 'port1');
  expect(southInterface).toBeTruthy();
  expect(southInterface.status).toBeUndefined();
  expect(southInterface.configured_status).toBeUndefined();
  expect(southInterface.configured).toBeUndefined();
  await page.locator('#topology-device-list .topology-device[data-node-id="south"]').click();
  await openInspectorGroup(page, 'topology');
  const port = page.locator('#topology-inspector .topology-port-detail[data-port-detail="port1"]');
  const toggle = port.locator('.topology-port-toggle');
  await toggle.focus();
  await page.keyboard.press('Enter');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  await expect(port.locator('.topology-port-detail-body')).toBeVisible();
  await expect(port).toContainText('Configured status · Unavailable');
  await expect(toggle).toHaveCSS('min-height', '44px');
  expect(await toggle.evaluate((button) => button.getBoundingClientRect().height)).toBeGreaterThanOrEqual(40);
  const sizes = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  expect(sizes).toEqual({ viewport: 375, document: 375, body: 375 });
  expect(analyses()).toBe(1);
});
