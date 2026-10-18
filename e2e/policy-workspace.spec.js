const { test, expect } = require('@playwright/test');
const path = require('node:path');

async function api(page, url) {
  return page.evaluate(async (target) => {
    const response = await fetch(target, { credentials: 'same-origin' });
    return { status: response.status, body: await response.json() };
  }, url);
}

async function submitConfig(page, config) {
  const before = await api(page, '/api/snapshots');
  const oldIds = new Set(before.body.snapshots.map(row => row.id));
  await page.locator('#config-input').fill(config);
  await expect(page.locator('#config-input')).toHaveValue(/\S/);
  await expect(page.locator('#import-button')).toBeEnabled();
  await page.locator('#import-button').click();
  let snapshotId;
  await expect.poll(async () => {
    const current = await api(page, '/api/snapshots');
    snapshotId = current.body.snapshots.find(row => !oldIds.has(row.id))?.id;
    return Boolean(snapshotId);
  }, { timeout: 10000 }).toBe(true);
  await expect(page.locator('#chain-select')).toBeEnabled();
  const response = await api(page, `/api/snapshots/${encodeURIComponent(snapshotId)}`);
  expect(response.status).toBe(200);
  return response.body.snapshot;
}

async function importServiceSample(page, includeUnknown = false) {
  await page.goto('/');
  await page.locator('#load-service-sample').click();
  await expect(page.locator('#config-input')).toHaveValue(/object-group service/i);
  if (includeUnknown) {
    const source = await page.locator('#config-input').inputValue();
    const injected = 'access-list SERVICE-IN extended deny tcp object-group MISSING-SOURCES any eq 8443\n';
    await page.locator('#config-input').fill(source.replace('access-group SERVICE-IN in interface outside', `${injected}access-group SERVICE-IN in interface outside`));
  }
  const config = await page.locator('#config-input').inputValue();
  return submitConfig(page, config);
}

async function evaluate(page, snapshot, flow, disposition) {
  await page.locator('#chain-select').selectOption(snapshot.chains[0].id);
  await page.locator('#flow-source').fill(flow.source || '192.0.2.42');
  await page.locator('#flow-destination').fill(flow.destination || '10.20.4.15');
  await page.locator('#flow-protocol').selectOption(flow.protocol || 'tcp');
  await page.locator('#flow-source-port').fill(String(flow.source_port || 52000));
  await page.locator('#flow-destination-port').fill(String(flow.destination_port));
  await page.locator('#evaluate-button').click();
  await expect(page.locator('#result-content')).toBeVisible({ timeout: 10000 });
  if (disposition === 'indeterminate') {
  }
  await expect(page.locator('#decision')).toContainText(new RegExp(disposition, 'i'), { timeout: 10000 });
}

test('service workspace searches and inspects inventory, explains decisions, and restores history', async ({ page }, testInfo) => {
  const snapshot = await importServiceSample(page, true);
  expect(snapshot.parser_version).toBe('asa-config-v2');
  expect(snapshot.objects.length).toBeGreaterThan(0);
  expect(snapshot.rule_origins.length).toBe(snapshot.chains[0].rules.length);
  await page.locator('#chain-select').selectOption(snapshot.chains[0].id);
  await expect(page.locator('#inventory-scope')).toContainText(snapshot.id);

  const totalText = await page.locator('#inventory-count').innerText();
  await page.locator('#inventory-search').fill('not-a-real-rule');
  await expect(page.locator('#inventory-empty')).toContainText(/no rules match/i);
  await expect(page.locator('#inventory-count')).toContainText(/^0 filtered · \d+ total/);
  await expect(page.locator('#inventory-count')).not.toHaveText(totalText);
  await expect(page.locator('#inventory-scope')).toContainText(snapshot.id);
  await page.locator('#inventory-search').fill('');
  const object = snapshot.objects.find((candidate) => candidate.kind === 'service_group' && candidate.members.length > 0);
  expect(object, 'sample has a populated service group').toBeTruthy();
  await page.locator('#object-search').fill(object.name);
  await page.locator('#object-list').getByText(object.name, { exact: true }).click();
  await expect(page.locator('#object-inspector')).toContainText(object.name);
  await expect(page.locator('#object-inspector')).toContainText(/lines \d+/i);

  const rules = snapshot.chains[0].rules;
  const unresolvedRule = rules.find((r) => (snapshot.rule_origins.find(origin => origin.rule_id === r.id)?.source_object_ids || []).includes('network_group:MISSING-SOURCES'));
  expect(unresolvedRule).toBeTruthy();
  await page.locator('#inventory-body').locator(`[data-rule-id="${unresolvedRule.id}"]`).click();
  await expect(page.locator('#rule-inspector')).toContainText('MISSING-SOURCES');
  await expect(page.locator('#rule-inspector')).toContainText(`physical line ${snapshot.rule_origins.find(origin => origin.rule_id === unresolvedRule.id).line}`);
  const outcomes = [
    { rule: rules.find((r) => r.action === 'permit'), port: 443, result: 'permit' },
    { rule: rules.find((r) => r.action === 'deny'), port: 22, result: 'deny' },
    { rule: rules.find((r) => (snapshot.rule_origins.find(origin => origin.rule_id === r.id)?.source_object_ids || []).some(id => id.includes('MISSING-SOURCES'))), port: 8443, source_port: 1, result: 'indeterminate' },
  ];
  for (const item of outcomes) {
    expect(item.rule, `sample has a ${item.result} candidate rule`).toBeTruthy();
    await evaluate(page, snapshot, { destination_port: item.port, source_port: item.source_port }, item.result);
    await expect(page.locator('#predicate-trace')).toContainText(/match|not match|unresolved/i);
  }
  const historyCount = await page.locator('#history-list button').count();
  expect(historyCount).toBeGreaterThanOrEqual(3);
  await page.reload();
  await expect(page.locator('#history-list button')).toHaveCount(historyCount);
  await page.locator('#history-list button').first().click();
  await expect(page.locator('#decision')).toContainText(/permit|deny|indeterminate/i);
  await page.locator('#object-search').fill(object.name);
  await page.locator('#object-list').getByText(object.name, { exact: true }).click();
  await page.screenshot({ path: path.resolve('test-results', 'policy-workspace.png'), fullPage: true });
});

test('an unresolved service deny before a later permit remains indeterminate', async ({ page }) => {
  await page.goto('/');
  await page.locator('#load-service-sample').click();
  await expect(page.locator('#config-input')).toHaveValue(/object-group service/i);
  const source = await page.locator('#config-input').inputValue();
  const earlyDeny = 'access-list SERVICE-IN extended deny object-group MISSING-SERVICES any any\n';
  const config = source.replace('access-list SERVICE-IN extended permit object-group WEB-APPS', `${earlyDeny}access-list SERVICE-IN extended permit object-group WEB-APPS`);
  expect(config).toContain('MISSING-SERVICES');
  const snapshot = await submitConfig(page, config);
  await page.locator('#chain-select').selectOption(snapshot.chains[0].id);
  const unknown = snapshot.chains[0].rules.find((rule) =>
    (snapshot.rule_origins.find(origin => origin.rule_id === rule.id)?.service_object_ids || []).includes('service_group:MISSING-SERVICES'));
  expect(unknown, 'missing service dependency is retained at the early deny').toBeTruthy();
  const laterPermit = snapshot.chains[0].rules.find((rule) => rule.action === 'permit' && rule.destination_port === '443');
  expect(laterPermit, 'a later explicit HTTPS permit remains in the chain').toBeTruthy();
  expect(unknown.position).toBeLessThan(laterPermit.position);
  await evaluate(page, snapshot, { destination_port: 443 }, 'indeterminate');
  await page.locator('#trace-body .trace-row.is-candidate').click();
  await expect(page.locator('#predicate-trace')).toContainText(/unresolved/i);
});

test('rule inventory pages large chains and search leaves flow evaluation unchanged', async ({ page }) => {
  await page.goto('/');
  const aceLines = Array.from({ length: 105 }, (_, index) =>
    `access-list PAGE-IN extended permit tcp any any eq ${2000 + index}`);
  const config = ['hostname paging-fw', ...aceLines, 'access-group PAGE-IN in interface outside', ''].join('\n');
  await page.locator('#config-input').fill(config);
  const snapshot = await submitConfig(page, config);
  await page.locator('#chain-select').selectOption(snapshot.chains[0].id);
  await expect(page.locator('#inventory-count')).toContainText('105 filtered · 105 total · page 1 of 2');
  await expect(page.locator('#inventory-next')).toBeEnabled();
  await page.locator('#inventory-next').click();
  await expect(page.locator('#inventory-count')).toContainText('105 filtered · 105 total · page 2 of 2');
  await expect(page.locator('#inventory-prev')).toBeEnabled();
  await page.locator('#inventory-prev').click();
  await page.locator('#flow-destination-port').fill('2104');
  await page.locator('#inventory-search').fill('2104');
  await expect(page.locator('#inventory-count')).toContainText('1 filtered · 105 total');
  await expect(page.locator('#inventory-body')).toContainText('2104');
  const flowBefore = await Promise.all(['#flow-source', '#flow-destination', '#flow-protocol', '#flow-source-port', '#flow-destination-port'].map(selector => page.locator(selector).inputValue()));
  await page.locator('#evaluate-button').click();
  await expect(page.locator('#decision')).toContainText(/permit/i);
  await expect(page.locator('#trace-count')).toContainText(/105 rules · page 1 of 2/i);
  await expect(page.locator('#trace-body tr')).toHaveCount(100);
  await page.locator('#trace-next').click();
  await expect(page.locator('#trace-count')).toContainText(/105 rules · page 2 of 2/i);
  await expect(page.locator('#trace-body tr')).toHaveCount(5);
  await expect(page.locator('#trace-body')).toContainText(snapshot.chains[0].rules[104].id);
  const flowAfter = await Promise.all(['#flow-source', '#flow-destination', '#flow-protocol', '#flow-source-port', '#flow-destination-port'].map(selector => page.locator(selector).inputValue()));
  expect(flowAfter).toEqual(flowBefore);
});

test('long-span dependency evidence paginates source with exact physical line ranges', async ({ page }) => {
  await page.goto('/');
  const filler = Array.from({ length: 205 }, (_, index) => `! retained policy evidence filler ${index + 1}`);
  const config = [
    'hostname long-evidence-fw',
    'object network WEB-SERVER',
    ' host 10.20.4.15',
    'access-list LONG-IN extended permit tcp any object WEB-SERVER eq 443',
    ...filler,
    'access-group LONG-IN in interface outside',
    '',
  ].join('\n');
  const snapshot = await submitConfig(page, config);
  const rule = snapshot.chains[0].rules.find(candidate => candidate.action === 'permit');
  expect(rule).toBeTruthy();
  const evidence = snapshot.evidence.filter(item => rule.evidence_refs.includes(item.id))
    .sort((a, b) => (b.end_line - b.start_line) - (a.end_line - a.start_line))[0];
  expect(evidence.end_line - evidence.start_line + 1).toBeGreaterThan(200);
  await evaluate(page, snapshot, { destination_port: 443 }, 'permit');
  const card = page.locator('#evidence-cards .evidence-card').filter({ hasText: `L${evidence.start_line}–${evidence.end_line}` });
  await card.getByRole('button', { name: /open source lines/i }).click();
  await expect(page.locator('#source-lines')).toBeVisible();
  await expect(page.locator('#source-pagination')).toContainText(/Next lines/);
  const readPage = async () => page.locator('#source-text .source-row .line-number').evaluateAll(nodes => nodes.map(node => Number(node.textContent.trim())));
  let shown = await readPage();
  expect(shown.length).toBeLessThanOrEqual(200);
  expect(shown[0]).toBe(evidence.start_line);
  expect(shown.at(-1)).toBe(evidence.start_line + shown.length - 1);
  await page.locator('#source-page-next').click();
  shown = await readPage();
  expect(shown.length).toBeLessThanOrEqual(200);
  expect(shown[0]).toBe(evidence.start_line + 200);
  expect(shown.at(-1)).toBe(evidence.end_line);
  await page.locator('#source-page-prev').click();
  expect((await readPage())[0]).toBe(evidence.start_line);
});

test('service inventory remains usable on mobile and renders hostile object names inert', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await page.locator('#load-service-sample').click();
  await expect(page.locator('#config-input')).toHaveValue(/object-group service/i);
  const hostile = '<img>';
  const source = await page.locator('#config-input').inputValue();
  const snapshot = await submitConfig(page, source.replaceAll('WEB-HTTPS', hostile));
  await page.locator('#object-search').fill('img');
  await expect(page.locator('#object-list')).toContainText(hostile);
  await page.locator('#object-list').getByText(hostile, { exact: true }).click();
  await page.locator('#chain-select').selectOption(snapshot.chains[0].id);
  await page.locator('#flow-destination-port').fill('443');
  await page.locator('#evaluate-button').click();
  await expect(page.locator('#decision')).toContainText(/permit/i);
  expect(await page.locator('img[src="x"]').count()).toBe(0);
  expect(await page.evaluate(() => window.__xssExecuted === true)).toBe(false);
  const width = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  expect(width.document, JSON.stringify(width)).toBeLessThanOrEqual(width.viewport);
  expect(width.body, JSON.stringify(width)).toBeLessThanOrEqual(width.viewport);
  expect(await page.locator('img[src="x"]').count()).toBe(0);
  await page.screenshot({ path: path.resolve('test-results', 'policy-workspace-mobile.png'), fullPage: true });
  expect(snapshot.objects.length).toBeGreaterThan(0);
});
