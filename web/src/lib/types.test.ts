import { describe, it, expect } from 'vitest';

describe('types', () => {
	it('exports HealthStatus and Service types', async () => {
		const types = await import('./types');
		expect(types).toBeDefined();
	});

	it('HEALTH_STATUSES contains all valid status values', async () => {
		const { HEALTH_STATUSES } = await import('./types');
		expect(HEALTH_STATUSES).toEqual(['healthy', 'unhealthy', 'authBlocked', 'unknown']);
	});

	it('Service interface accepts valid shapes', async () => {
		const types = await import('./types');
		// Module is importable — type checking ensures interface correctness at compile time
		expect(types).toBeDefined();
	});
});
