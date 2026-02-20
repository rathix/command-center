import { describe, it, expect } from 'vitest';
import { HEALTH_STATUSES } from './types';
import type { HealthStatus, Service } from './types';

describe('types', () => {
	it('HEALTH_STATUSES contains all valid status values', () => {
		expect(HEALTH_STATUSES).toEqual(['healthy', 'unhealthy', 'authBlocked', 'unknown']);
	});

	it('HealthStatus type aligns with HEALTH_STATUSES', () => {
		const statuses: HealthStatus[] = ['healthy', 'unhealthy', 'authBlocked', 'unknown'];
		expect(statuses).toEqual(HEALTH_STATUSES);
	});

	it('Service interface matches the SSE data model shape', () => {
		const service: Service = {
			name: 'api',
			displayName: 'api',
			namespace: 'default',
			url: 'https://api.example.local',
			status: 'healthy',
			httpCode: 200,
			responseTimeMs: 42,
			lastChecked: '2026-02-20T00:00:00Z',
			lastStateChange: '2026-02-20T00:00:00Z',
			errorSnippet: null
		};

		expect(service).toHaveProperty('name');
		expect(service).toHaveProperty('displayName');
		expect(service).toHaveProperty('namespace');
		expect(service).toHaveProperty('url');
		expect(service).toHaveProperty('status');
		expect(service).toHaveProperty('httpCode');
		expect(service).toHaveProperty('responseTimeMs');
		expect(service).toHaveProperty('lastChecked');
		expect(service).toHaveProperty('lastStateChange');
		expect(service).toHaveProperty('errorSnippet');
	});
});
