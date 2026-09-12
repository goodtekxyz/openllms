/**
 * Capture console hub UI screenshots.
 * Usage: node scripts/capture-console-ui.mjs [baseUrl]
 */
import { chromium } from 'playwright';
import { mkdir } from 'fs/promises';
import path from 'path';

const base = (process.argv[2] || 'http://127.0.0.1:8765').replace(/\/$/, '');
const consolePath = process.env.CONSOLE_PATH ||
  (base.includes('127.0.0.1') || base.includes('localhost') ? '/console.html' : '/console');
const outDir = process.env.SCREENSHOT_DIR || path.resolve('artifacts/screenshots');

const overviewNone = {
  billing: {
    plan: 'none',
    entitled: false,
    limits: {},
    usage_month: { tokens_total: 1540489, soft_cap_tokens: null },
  },
  accounts: [
    { id: 'a1', vendor: 'codex', name: 'work', auth_type: 'oauth', health: 'ok', glyph: '●', quota_remaining_pct: 82 },
    { id: 'a2', vendor: 'claude', name: 'main', auth_type: 'oauth', health: 'ok', glyph: '●', quota_remaining_pct: 64 },
    { id: 'a3', vendor: 'openai', name: 'api', auth_type: 'api_key', health: 'ok', glyph: '●', quota_remaining_pct: null },
  ],
  routes: [
    {
      slug: 'codex-quota-first', preset: 'quota-first', strategy: 'quota_aware',
      account_ids: ['a1'], account_refs: ['codex:work'], openai_base: '/r/codex-quota-first/v1',
    },
    {
      slug: 'claude-failover', preset: 'failover', strategy: 'failover',
      account_ids: ['a2'], account_refs: ['claude:main'], openai_base: '/r/claude-failover/v1',
    },
  ],
  keys: Array.from({ length: 11 }, (_, i) => ({
    id: 'k' + i, name: 'key-' + i, key_prefix: 'sk-gt-ab', route_id: 'codex-quota-first', revoked: false,
  })),
  public_base_url: base,
};

const overviewPro = {
  ...overviewNone,
  billing: {
    plan: 'pro',
    entitled: true,
    limits: { accounts: 10, routes: 5, keys: 20, rpm: 60, soft_cap_tokens: 5000000 },
    usage_month: { tokens_total: 1540489, soft_cap_tokens: 5000000 },
  },
};

async function shot(page, name) {
  const file = path.join(outDir, name);
  await page.screenshot({ path: file, fullPage: true });
  console.log('saved', file);
}

async function stubOverview(page, payload) {
  await page.unroute('**/console/api/**').catch(() => {});
  await page.route('**/console/api/**', async (route) => {
    const url = route.request().url();
    if (url.includes('/overview')) {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(payload) });
      return;
    }
    if (url.includes('/me')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ login: 'preview' }),
      });
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
  });
}

async function openHub(page, payload) {
  await stubOverview(page, payload);
  await page.goto(base + consolePath, { waitUntil: 'networkidle' });
  await page.waitForTimeout(400);
  // Ensure hub is painted (local static has no nav-account)
  await page.evaluate(() => {
    const err = document.getElementById('err');
    if (err) err.classList.add('hidden');
    const nav = document.getElementById('nav-account');
    if (nav) nav.textContent = '@preview';
  });
  await page.waitForSelector('#app-view:not(.hidden)', { timeout: 5000 });
  await page.waitForTimeout(200);
}

async function main() {
  await mkdir(outDir, { recursive: true });
  const browser = await chromium.launch({ headless: true });

  // Login (no stub → 401)
  {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await ctx.newPage();
    await page.route('**/console/api/**', (route) =>
      route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"unauthorized"}' }));
    await page.goto(base + consolePath, { waitUntil: 'networkidle' });
    await page.waitForTimeout(400);
    await shot(page, 'console-hub-login-light.png');
    await page.emulateMedia({ colorScheme: 'dark' });
    await page.evaluate(() => document.documentElement.classList.add('dark'));
    await page.waitForTimeout(200);
    await shot(page, 'console-hub-login-dark.png');
    await ctx.close();
  }

  // Overview — plan needed (dark + light)
  {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await ctx.newPage();
    await page.emulateMedia({ colorScheme: 'dark' });
    await page.addInitScript(() => document.documentElement.classList.add('dark'));
    await openHub(page, overviewNone);
    await shot(page, 'console-hub-overview-dark.png');

    await page.emulateMedia({ colorScheme: 'light' });
    await page.evaluate(() => document.documentElement.classList.remove('dark'));
    await page.waitForTimeout(150);
    await shot(page, 'console-hub-overview-light.png');

    await page.locator('.console-tab[data-tab="accounts"]').click();
    await page.waitForTimeout(200);
    await shot(page, 'console-hub-accounts-light.png');

    await page.evaluate(() => document.getElementById('modal-connect')?.classList.remove('hidden'));
    await page.waitForTimeout(100);
    await shot(page, 'console-hub-modal-light.png');
    await ctx.close();
  }

  // Entitled overview
  {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await ctx.newPage();
    await openHub(page, overviewPro);
    await shot(page, 'console-hub-overview-entitled-light.png');
    await ctx.close();
  }

  // Mobile dark
  {
    const ctx = await browser.newContext({ viewport: { width: 390, height: 844 } });
    const page = await ctx.newPage();
    await page.emulateMedia({ colorScheme: 'dark' });
    await page.addInitScript(() => document.documentElement.classList.add('dark'));
    await openHub(page, overviewNone);
    await shot(page, 'console-hub-overview-mobile-dark.png');
    await ctx.close();
  }

  await browser.close();
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
