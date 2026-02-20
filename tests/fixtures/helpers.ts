import { expect, Page } from '@playwright/test';

export const waitForDashboardLoad = async (page: Page) => {
  await expect(page.getByRole('link', { name: 'Skip to service list' })).toBeVisible();
};
