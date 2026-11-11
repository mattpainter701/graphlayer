const { test, expect } = require('@playwright/test');
const fs = require('node:fs/promises');

async function inspectDownload(page, download) {
  expect(download.suggestedFilename()).toMatch(/^graphlayer-network-(map|topology)\.svg$/);
  const content = await fs.readFile(await download.path(), 'utf8');
  return page.evaluate((xml) => {
    const doc = new DOMParser().parseFromString(xml, 'image/svg+xml');
    const root = doc.documentElement;
    return {
      parseError: Boolean(doc.querySelector('parsererror')),
      root: root.localName,
      namespace: root.namespaceURI,
      title: root.querySelector(':scope > title')?.textContent,
      description: root.querySelector(':scope > desc')?.textContent,
      viewBox: root.getAttribute('viewBox'),
      width: Number(root.getAttribute('width')),
      height: Number(root.getAttribute('height')),
      text: root.textContent,
      styledShapes: root.querySelectorAll('rect[style*="fill:"], path[style*="stroke:"]').length,
      forbidden: root.querySelectorAll('script, foreignObject, iframe, image, a').length,
      unsafeAttributes: [...root.querySelectorAll('*')].flatMap((element) => [...element.attributes]
        .filter((attribute) => /^on|^data-/i.test(attribute.name) || (/href$/.test(attribute.name) && !attribute.value.startsWith('#')))
        .map((attribute) => attribute.name)),
    };
  }, content);
}

async function loadMap(page) {
  let projections = 0;
  page.on('request', (request) => { if (request.url().endsWith('/api/topology-layers')) projections += 1; });
  await page.goto('/#network-map');
  await expect(page.locator('#tl-export-view')).toBeDisabled();
  await page.locator('#tl-load').click();
  await expect(page.locator('#tl-summary')).toContainText('1000');
  await page.locator('#tl-vrf').fill('lab-vrf-01');
  await page.locator('#tl-vdom').fill('campus');
  await page.locator('#tl-apply-filters').click();
  await expect(page.locator('#tl-host-results-summary')).toContainText('100 matching hosts');
  await expect(page.locator('#tl-export-view')).toBeEnabled();
  return () => projections;
}

async function loadPath(page) {
  let analyses = 0;
  page.on('request', (request) => {
    if (request.method() === 'POST' && /\/api\/(network-explorations|topology-view)$/.test(request.url())) analyses += 1;
  });
  await page.goto('http://127.0.0.1:8089/#network-path');
  await page.locator('#network-load-exploration-sample').click();
  await expect(page.locator('#network-status')).toContainText('4 sample firewall snapshots imported');
  await page.locator('#network-explore').click();
  await expect(page.locator('#exploration-result')).toBeVisible();
  await expect(page.locator('#topology-export-view')).toBeEnabled();
  return () => analyses;
}

test('network map exports a self-contained rendered SVG with the applied scope and no new projection', async ({ page }) => {
  const requests = await loadMap(page);
  const before = await page.locator('#tl-svg').evaluate((svg) => ({ viewBox: svg.getAttribute('viewBox'), content: svg.innerHTML }));
  const beforeRequests = requests();
  const pending = page.waitForEvent('download');
  await page.locator('#tl-export-view').click();
  const exported = await inspectDownload(page, await pending);
  expect(exported).toMatchObject({ parseError: false, root: 'svg', namespace: 'http://www.w3.org/2000/svg', title: 'GraphLayer network map', forbidden: 0, unsafeAttributes: [], viewBox: before.viewBox });
  expect(exported.width).toBeGreaterThan(0);
  expect(exported.height).toBeGreaterThan(0);
  expect(exported.styledShapes).toBeGreaterThan(0);
  expect(exported.description).toContain('VRF: lab-vrf-01');
  expect(exported.description).toContain('VDOM: campus');
  expect(exported.description).toContain('does not establish live reachability');
  expect(await page.locator('#tl-svg').evaluate((svg) => svg.innerHTML)).toBe(before.content);
  expect(requests()).toBe(beforeRequests);
});

test('saved path SVG keeps the selected configured view and exported text is XML-safe', async ({ page }) => {
  const requests = await loadPath(page);
  await page.locator('#topology-path-next').click();
  const beforeRequests = requests();
  const beforeViewBox = await page.locator('#exploration-topology').getAttribute('viewBox');
  const pending = page.waitForEvent('download');
  await page.locator('#topology-export-view').click();
  const exported = await inspectDownload(page, await pending);
  expect(exported).toMatchObject({ parseError: false, title: 'GraphLayer network topology', forbidden: 0, unsafeAttributes: [], viewBox: beforeViewBox });
  expect(exported.text).toContain('branch-edge');
  expect(exported.text).toContain('ethernet1/2');
  expect(exported.description).toContain('Configured branch');
  expect(exported.description).toContain('not live traffic');
  expect(exported.styledShapes).toBeGreaterThan(0);
  expect(requests()).toBe(beforeRequests);
  // A metadata label is serialized as text; it cannot become executable SVG markup.
  const pendingLabel = page.waitForEvent('download');
  await page.evaluate(() => window.GraphLayerGraphExport.download(document.querySelector('#exploration-topology'), {
    filename: 'graphlayer-network-topology.svg', title: '<script>label & comparison</script>', description: 'Configuration < observation & model',
  }));
  const labeled = await inspectDownload(page, await pendingLabel);
  expect(labeled.parseError).toBe(false);
  expect(labeled.title).toBe('<script>label & comparison</script>');
  expect(labeled.description).toBe('Configuration < observation & model');
  expect(labeled.forbidden).toBe(0);
});

test('export failures report a status and preserve the current map and path', async ({ page }) => {
  const mapRequests = await loadMap(page);
  const mapBefore = await page.locator('#tl-svg').innerHTML();
  const count = mapRequests();
  await page.evaluate(() => { window.GraphLayerGraphExport = { download() { throw new Error('Synthetic export failure'); } }; });
  await page.locator('#tl-export-view').click();
  await expect(page.locator('#tl-caption')).toContainText('SVG export failed');
  expect(await page.locator('#tl-svg').innerHTML()).toBe(mapBefore);
  expect(mapRequests()).toBe(count);
  const pathRequests = await loadPath(page);
  const pathBefore = await page.locator('#exploration-topology').innerHTML();
  const pathCount = pathRequests();
  await page.evaluate(() => { window.GraphLayerGraphExport = { download() { throw new Error('Synthetic export failure'); } }; });
  await page.locator('#topology-export-view').click();
  await expect(page.locator('.topology-stage-caption')).toContainText('SVG export failed');
  expect(await page.locator('#exploration-topology').innerHTML()).toBe(pathBefore);
  expect(pathRequests()).toBe(pathCount);
});
