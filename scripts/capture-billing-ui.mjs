/**
 * Capture billing UI screenshots (signed-out, mock status, mobile, dark).
 * Usage: node scripts/capture-billing-ui.mjs [baseUrl]
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

  await page.goto(base + '/billing', { waitUntil: 'networkidle' });
  await page.waitForTimeout(600);
  await shot(page, 'billing-signed-out-light.png');

  await page.emulateMedia({ colorScheme: 'dark' });
  await page.evaluate(() => document.documentElement.classList.add('dark'));
  await page.waitForTimeout(300);
  await shot(page, 'billing-signed-out-dark.png');

  await page.evaluate(() => {
    document.getElementById('auth-hint').classList.add('hidden');
    document.getElementById('status-box').innerHTML =
      '<h2>Plan status</h2><dl>' +
      '<dt>plan</dt><dd>trial</dd>' +
      '<dt>status</dt><dd>active</dd>' +
      '<dt>entitled</dt><dd>true</dd>' +
      '<dt>trial used</dt><dd>true</dd>' +
      '<dt>period end</dt><dd>2026-08-30</dd>' +
      '<dt>provider</dt><dd>mock</dd>' +
      '<dt>limits</dt><dd>acc 5 · routes 2 · keys 2 · rpm 60</dd>' +
      '<dt>usage</dt><dd>tokens 1200 / 5000000</dd>' +
      '<dt>rails</dt><dd>polar=true unifi=true mock=true</dd>' +
      '</dl>';
    const btn = document.getElementById('btn-signin');
    btn.textContent = 'Signed in';
    btn.disabled = true;
    document.getElementById('btn-trial').disabled = true;
  });
  await shot(page, 'billing-status-mock-dark.png');

  await page.emulateMedia({ colorScheme: 'light' });
  await page.evaluate(() => document.documentElement.classList.remove('dark'));
  await shot(page, 'billing-status-mock-light.png');

  const mobile = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const mp = await mobile.newPage();
  await mp.goto(base + '/billing', { waitUntil: 'networkidle' });
  await mp.waitForTimeout(400);
  await mp.screenshot({ path: path.join(outDir, 'billing-signed-out-mobile-light.png'), fullPage: true });
  console.log('saved', path.join(outDir, 'billing-signed-out-mobile-light.png'));

  await browser.close();
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
