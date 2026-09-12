/**
 * Capture marketing + install + app chrome screenshots for UX audit.
 * Usage: from /tmp/pw-capture: SCREENSHOT_DIR=... node capture-site-ui.mjs [baseUrl]
 */
import { chromium } from 'playwright';
import { mkdir } from 'fs/promises';
import path from 'path';

const base = process.argv[2] || 'https://dev-llms.goodtek.xyz';
const outDir = process.env.SCREENSHOT_DIR || 'docs/artifacts/screenshots';

async function shot(page, name) {
  const file = path.join(outDir, name);
  await page.screenshot({ path: file, fullPage: true });
  console.log('saved', file);
}

async function main() {
  await mkdir(outDir, { recursive: true });
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();

  for (const [route, name] of [
    ['/', 'site-landing-ko.png'],
    ['/install', 'site-install-ko.png'],
    ['/en', 'site-landing-en.png'],
    ['/en/install', 'site-install-en.png'],
    ['/console', 'site-console.png'],
    ['/billing', 'site-billing.png'],
    ['/admin', 'site-admin.png'],
  ]) {
    await page.goto(base + route, { waitUntil: 'networkidle' });
    await page.waitForTimeout(500);
    await shot(page, name);
  }

  // Install copy buttons visible
  await page.goto(base + '/install', { waitUntil: 'networkidle' });
  await page.waitForTimeout(400);
  const n = await page.locator('.copy-btn').count();
  console.log('copy buttons on /install:', n);
  await page.locator('.copy-btn').first().click();
  await page.waitForTimeout(300);
  await shot(page, 'site-install-ko-copy-clicked.png');

  const mobile = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const mp = await mobile.newPage();
  await mp.goto(base + '/install', { waitUntil: 'networkidle' });
  await mp.waitForTimeout(400);
  await mp.screenshot({ path: path.join(outDir, 'site-install-ko-mobile.png'), fullPage: true });
  console.log('saved', path.join(outDir, 'site-install-ko-mobile.png'));

  await browser.close();
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
