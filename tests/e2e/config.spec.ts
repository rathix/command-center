import { test, expect } from '@playwright/test';

test.describe('Server Configuration E2E', () => {
  test('[P1] should respond on custom configured port', async ({ page }) => {
    const customPort = process.env.LISTEN_ADDR || ':8443';
    const url = `https://localhost${customPort}`;
    await page.goto(url);
    await expect(page).toHaveTitle(/Command Center/);
    await expect(page.getByRole('link', { name: 'Skip to service list' })).toBeVisible();
  });
});
