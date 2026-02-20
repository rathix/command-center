import { describe, it, expect, beforeEach } from 'vitest';
import {
	getSortedServices,
	getGroupedServices,
	getCounts,
	getHasProblems,
	getConnectionStatus,
	getLastUpdated,
	replaceAll,
	addOrUpdate,
	remove,
	setConnectionStatus,
	_resetForTesting
} from './serviceStore.svelte';
import type { Service } from './types';

function makeService(overrides: Partial<Service> & { name: string }): Service {
	return {
		displayName: overrides.displayName ?? overrides.name,
		namespace: 'default',
		url: 'https://test.example.com',
		status: 'unknown',
		httpCode: null,
		responseTimeMs: null,
		lastChecked: null,
		lastStateChange: null,
		errorSnippet: null,
		...overrides
	};
}

beforeEach(() => {
	_resetForTesting();
});

describe('serviceStore', () => {
	describe('replaceAll', () => {
		it('replaces entire services map with new data', () => {
			replaceAll([makeService({ name: 'svc-a' }), makeService({ name: 'svc-b' })]);
			expect(getSortedServices()).toHaveLength(2);
		});

		it('clears previous data when replacing', () => {
			replaceAll([makeService({ name: 'old' })]);
			replaceAll([makeService({ name: 'new' })]);
			expect(getSortedServices()).toHaveLength(1);
			expect(getSortedServices()[0].name).toBe('new');
		});

		it('handles empty array', () => {
			replaceAll([makeService({ name: 'existing' })]);
			replaceAll([]);
			expect(getSortedServices()).toHaveLength(0);
			expect(getCounts().total).toBe(0);
		});

		it('keys services by namespace/name', () => {
			replaceAll([
				makeService({ name: 'svc', namespace: 'ns1' }),
				makeService({ name: 'svc', namespace: 'ns2' })
			]);
			expect(getSortedServices()).toHaveLength(2);
		});
	});

	describe('addOrUpdate', () => {
		it('adds a new service when key not present', () => {
			addOrUpdate(makeService({ name: 'new-svc' }));
			expect(getSortedServices()).toHaveLength(1);
			expect(getSortedServices()[0].name).toBe('new-svc');
		});

		it('updates existing service when key present', () => {
			addOrUpdate(makeService({ name: 'svc', status: 'unknown' }));
			addOrUpdate(makeService({ name: 'svc', status: 'healthy' }));
			expect(getSortedServices()).toHaveLength(1);
			expect(getSortedServices()[0].status).toBe('healthy');
		});
	});

	describe('remove', () => {
		it('removes service by namespace/name', () => {
			replaceAll([
				makeService({ name: 'svc-a', namespace: 'default' }),
				makeService({ name: 'svc-b', namespace: 'default' })
			]);
			remove('default', 'svc-a');
			expect(getSortedServices()).toHaveLength(1);
			expect(getSortedServices()[0].name).toBe('svc-b');
		});

		it('is a no-op for non-existent key', () => {
			replaceAll([makeService({ name: 'existing' })]);
			remove('default', 'non-existent');
			expect(getSortedServices()).toHaveLength(1);
		});
	});

	describe('sortedServices', () => {
		it('sorts problems first: unhealthy → authBlocked → unknown → healthy', () => {
			replaceAll([
				makeService({ name: 'healthy-svc', status: 'healthy' }),
				makeService({ name: 'unknown-svc', status: 'unknown' }),
				makeService({ name: 'blocked-svc', status: 'authBlocked' }),
				makeService({ name: 'unhealthy-svc', status: 'unhealthy' })
			]);
			expect(getSortedServices().map((s) => s.status)).toEqual([
				'unhealthy',
				'authBlocked',
				'unknown',
				'healthy'
			]);
		});

		it('sorts alphabetically within each status group', () => {
			replaceAll([
				makeService({ name: 'zebra', status: 'healthy' }),
				makeService({ name: 'alpha', status: 'healthy' }),
				makeService({ name: 'mid', status: 'healthy' })
			]);
			expect(getSortedServices().map((s) => s.name)).toEqual(['alpha', 'mid', 'zebra']);
		});

		it('sorts case-insensitively within groups', () => {
			replaceAll([
				makeService({ name: 'Bravo', status: 'healthy' }),
				makeService({ name: 'alpha', status: 'healthy' })
			]);
			expect(getSortedServices().map((s) => s.name)).toEqual(['alpha', 'Bravo']);
		});
	});

	describe('counts', () => {
		it('reflects current map state', () => {
			replaceAll([
				makeService({ name: 'h1', status: 'healthy' }),
				makeService({ name: 'h2', status: 'healthy' }),
				makeService({ name: 'u1', status: 'unhealthy' }),
				makeService({ name: 'a1', status: 'authBlocked' }),
				makeService({ name: 'k1', status: 'unknown' })
			]);
			expect(getCounts()).toEqual({
				total: 5,
				healthy: 2,
				unhealthy: 1,
				authBlocked: 1,
				unknown: 1
			});
		});

		it('returns all zeros for empty store', () => {
			expect(getCounts()).toEqual({
				total: 0,
				healthy: 0,
				unhealthy: 0,
				authBlocked: 0,
				unknown: 0
			});
		});
	});

	describe('hasProblems', () => {
		it('is true when unhealthy services exist', () => {
			replaceAll([makeService({ name: 'bad', status: 'unhealthy' })]);
			expect(getHasProblems()).toBe(true);
		});

		it('is true when authBlocked services exist', () => {
			replaceAll([makeService({ name: 'blocked', status: 'authBlocked' })]);
			expect(getHasProblems()).toBe(true);
		});

		it('is true when only healthy and unknown services exist', () => {
			replaceAll([
				makeService({ name: 'good', status: 'healthy' }),
				makeService({ name: 'new', status: 'unknown' })
			]);
			expect(getHasProblems()).toBe(true);
		});

		it('is false for empty store', () => {
			expect(getHasProblems()).toBe(false);
		});
	});

	describe('connectionStatus', () => {
		it('defaults to connecting', () => {
			expect(getConnectionStatus()).toBe('connecting');
		});

		it('can be set to connected', () => {
			setConnectionStatus('connected');
			expect(getConnectionStatus()).toBe('connected');
		});

		it('can be set to disconnected', () => {
			setConnectionStatus('disconnected');
			expect(getConnectionStatus()).toBe('disconnected');
		});

		it('transitions through states correctly', () => {
			expect(getConnectionStatus()).toBe('connecting');
			setConnectionStatus('connected');
			expect(getConnectionStatus()).toBe('connected');
			setConnectionStatus('disconnected');
			expect(getConnectionStatus()).toBe('disconnected');
			setConnectionStatus('connected');
			expect(getConnectionStatus()).toBe('connected');
		});
	});

	describe('groupedServices', () => {
		it('groups mixed health states correctly — unhealthy and authBlocked in needsAttention, healthy in healthy', () => {
			replaceAll([
				makeService({ name: 'good-svc', status: 'healthy' }),
				makeService({ name: 'bad-svc', status: 'unhealthy' }),
				makeService({ name: 'blocked-svc', status: 'authBlocked' })
			]);
			const groups = getGroupedServices();
			expect(groups.needsAttention.map((s) => s.name)).toEqual(['bad-svc', 'blocked-svc']);
			expect(groups.healthy.map((s) => s.name)).toEqual(['good-svc']);
		});

		it('sorts alphabetically within each group by displayName', () => {
			replaceAll([
				makeService({ name: 'zebra', status: 'healthy' }),
				makeService({ name: 'alpha', status: 'healthy' }),
				makeService({ name: 'delta', status: 'unhealthy' }),
				makeService({ name: 'bravo', status: 'unhealthy' })
			]);
			const groups = getGroupedServices();
			expect(groups.needsAttention.map((s) => s.name)).toEqual(['bravo', 'delta']);
			expect(groups.healthy.map((s) => s.name)).toEqual(['alpha', 'zebra']);
		});

		it('returns empty needsAttention when all services are healthy', () => {
			replaceAll([
				makeService({ name: 'alpha', status: 'healthy' }),
				makeService({ name: 'bravo', status: 'healthy' })
			]);
			const groups = getGroupedServices();
			expect(groups.needsAttention).toEqual([]);
			expect(groups.healthy.map((s) => s.name)).toEqual(['alpha', 'bravo']);
		});

		it('returns empty healthy when all services are unhealthy', () => {
			replaceAll([
				makeService({ name: 'bad-a', status: 'unhealthy' }),
				makeService({ name: 'bad-b', status: 'authBlocked' })
			]);
			const groups = getGroupedServices();
			expect(groups.needsAttention.map((s) => s.name)).toEqual(['bad-a', 'bad-b']);
			expect(groups.healthy).toEqual([]);
		});

		it('places unknown status services in needsAttention group', () => {
			replaceAll([
				makeService({ name: 'new-svc', status: 'unknown' }),
				makeService({ name: 'good-svc', status: 'healthy' })
			]);
			const groups = getGroupedServices();
			expect(groups.needsAttention.map((s) => s.name)).toEqual(['new-svc']);
			expect(groups.healthy.map((s) => s.name)).toEqual(['good-svc']);
		});
	});

	describe('groupedServices snapshot sort order', () => {
		it('does not physically move a service between sections when status changes via addOrUpdate', () => {
			replaceAll([
				makeService({ name: 'alpha', status: 'healthy' }),
				makeService({ name: 'bravo', status: 'healthy' }),
				makeService({ name: 'charlie', status: 'unhealthy' })
			]);
			// Initial order: charlie (unhealthy) in needsAttention; alpha, bravo (healthy) in healthy
			const initialGroups = getGroupedServices();
			expect(initialGroups.needsAttention.map((s) => s.name)).toEqual(['charlie']);
			expect(initialGroups.healthy.map((s) => s.name)).toEqual(['alpha', 'bravo']);

			// alpha transitions to unhealthy via SSE update — membership should NOT change to keep position stable
			addOrUpdate(makeService({ name: 'alpha', status: 'unhealthy' }));
			const updatedGroups = getGroupedServices();
			// alpha stays in the "healthy" group visually to prevent reordering
			expect(updatedGroups.needsAttention.map((s) => s.name)).toEqual(['charlie']);
			expect(updatedGroups.healthy.map((s) => s.name)).toEqual(['alpha', 'bravo']);
			expect(updatedGroups.healthy.find((s) => s.name === 'alpha')?.status).toBe('unhealthy');
		});

		it('recalculates sort order and grouping on replaceAll (SSE reconnect)', () => {
			replaceAll([
				makeService({ name: 'alpha', status: 'healthy' }),
				makeService({ name: 'bravo', status: 'unhealthy' })
			]);
			// bravo first (unhealthy), then alpha (healthy)
			expect(getGroupedServices().needsAttention.map((s) => s.name)).toEqual(['bravo']);
			expect(getGroupedServices().healthy.map((s) => s.name)).toEqual(['alpha']);

			// SSE reconnect — replaceAll with alpha now unhealthy
			replaceAll([
				makeService({ name: 'alpha', status: 'unhealthy' }),
				makeService({ name: 'bravo', status: 'healthy' })
			]);
			// Sort order recalculated: alpha (unhealthy) first, then bravo (healthy)
			expect(getGroupedServices().needsAttention.map((s) => s.name)).toEqual(['alpha']);
			expect(getGroupedServices().healthy.map((s) => s.name)).toEqual(['bravo']);
		});

		it('recalculates sort order when a new service appears via addOrUpdate', () => {
			replaceAll([makeService({ name: 'bravo', status: 'healthy' })]);
			expect(getGroupedServices().healthy.map((s) => s.name)).toEqual(['bravo']);

			// New service via addOrUpdate — triggers sort recalc since key is new
			addOrUpdate(makeService({ name: 'alpha', status: 'healthy' }));
			expect(getGroupedServices().healthy.map((s) => s.name)).toEqual(['alpha', 'bravo']);
		});
	});

	describe('lastUpdated', () => {
		it('is null initially', () => {
			expect(getLastUpdated()).toBeNull();
		});

		it('updates on replaceAll', () => {
			replaceAll([makeService({ name: 'svc' })]);
			expect(getLastUpdated()).toBeInstanceOf(Date);
		});

		it('updates on addOrUpdate', () => {
			addOrUpdate(makeService({ name: 'svc' }));
			expect(getLastUpdated()).toBeInstanceOf(Date);
		});

		it('updates on remove', () => {
			replaceAll([makeService({ name: 'svc' })]);
			remove('default', 'svc');
			expect(getLastUpdated()).toBeInstanceOf(Date);
		});
	});
});
