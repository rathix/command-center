import { test as base } from '@playwright/test';

export const test = base.extend({
  authenticatedUser: async ({ page }, use) => {
    await page.goto('/login');
    await page.fill('[name="email"]', 'test@example.com');
    await page.fill('[name="password"]', 'password');
    await page.click('button[type="submit"]');
    await page.waitForURL('/dashboard');
    await use(page);
  },
  authToken: async ({ request }, use) => {
    const response = await request.post('/api/auth/login', {
      data: { email: 'test@example.com', password: 'password' },
    });
    const { token } = await response.json();
    await use(token);
  },
});
