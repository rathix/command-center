import { describe, it, expect } from 'vitest';
import * as layout from './+layout.ts';

describe('+layout.ts SPA configuration', () => {
	it('disables SSR for SPA mode', () => {
		expect(layout.ssr).toBe(false);
	});

	it('enables CSR for client-side rendering', () => {
		expect(layout.csr).toBe(true);
	});

	it('disables prerendering for SPA mode', () => {
		expect(layout.prerender).toBe(false);
	});
});
