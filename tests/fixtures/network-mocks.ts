import { Page } from '@playwright/test';

export const mockK8sApiSuccess = async (page: Page) => {
  await page.route('/api/events', (route) => {
    route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      body: 'event: state\ndata: {"services": []}\n\n',
    });
  });
};
