import { describe, it, expect, beforeEach } from 'vitest';
import {
	getSortedServices,
	getGroupedServices,
	getCounts,
	getHasProblems,
	getConnectionStatus,
	getLastUpdated,
	getAppVersion,
	getK8sConnected,
	getK8sLastEvent,
	getHealthCheckIntervalMs,
	replaceAll,
	addOrUpdate,
	remove,
	setConnectionStatus,
	setK8sStatus,
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
	it('reset helper clears appVersion to avoid test state leakage', () => {
		replaceAll([makeService({ name: 'svc-a' })], 'v1.2.3');
		expect(getAppVersion()).toBe('v1.2.3');

		_resetForTesting();
		expect(getAppVersion()).toBe('');
	});

	describe('replaceAll', () => {
		it('replaces entire services map with new data', () => {
			replaceAll([makeService({ name: 'svc-a' }), makeService({ name: 'svc-b' })], 'v1.0.0');
			expect(getSortedServices()).toHaveLength(2);
			expect(getAppVersion()).toBe('v1.0.0');
		});

		it('clears previous data when replacing', () => {
			replaceAll([makeService({ name: 'old' })], 'v1');
			replaceAll([makeService({ name: 'new' })], 'v2');
			expect(getSortedServices()).toHaveLength(1);
			expect(getSortedServices()[0].name).toBe('new');
			expect(getAppVersion()).toBe('v2');
		});

		it('handles empty array', () => {
			replaceAll([makeService({ name: 'existing' })], 'v1');
			replaceAll([], 'v2');
			expect(getSortedServices()).toHaveLength(0);
			expect(getCounts().total).toBe(0);
			expect(getAppVersion()).toBe('v2');
		});

		it('keys services by namespace/name', () => {
			replaceAll([
				makeService({ name: 'svc', namespace: 'ns1' }),
				makeService({ name: 'svc', namespace: 'ns2' })
			], 'v1');
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
			], 'v1');
			remove('default', 'svc-a');
			expect(getSortedServices()).toHaveLength(1);
			expect(getSortedServices()[0].name).toBe('svc-b');
		});

		it('is a no-op for non-existent key', () => {
			replaceAll([makeService({ name: 'existing' })], 'v1');
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
			], 'v1.0.0');
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
			], 'v1.0.0');
			expect(getSortedServices().map((s) => s.name)).toEqual(['alpha', 'mid', 'zebra']);
		});

		it('sorts case-insensitively within groups', () => {
			replaceAll([
				makeService({ name: 'Bravo', status: 'healthy' }),
				makeService({ name: 'alpha', status: 'healthy' })
			], 'v1.0.0');
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
			], 'v1.0.0');
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
			replaceAll([makeService({ name: 'bad', status: 'unhealthy' })], 'v1.0.0');
			expect(getHasProblems()).toBe(true);
		});

		it('is true when authBlocked services exist', () => {
			replaceAll([makeService({ name: 'blocked', status: 'authBlocked' })], 'v1.0.0');
			expect(getHasProblems()).toBe(true);
		});

		it('is true when only healthy and unknown services exist', () => {
			replaceAll([
				makeService({ name: 'good', status: 'healthy' }),
				makeService({ name: 'new', status: 'unknown' })
			], 'v1.0.0');
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

		it('can be set to reconnecting', () => {
			setConnectionStatus('reconnecting');
			expect(getConnectionStatus()).toBe('reconnecting');
		});

		it('preserves service data when connection status changes to reconnecting', () => {
			const checkedAt = new Date().toISOString();
			replaceAll([
				makeService({ name: 'svc-a', status: 'healthy', lastChecked: checkedAt }),
				makeService({ name: 'svc-b', status: 'unhealthy' })
			], 'v1.0.0');
			expect(getSortedServices()).toHaveLength(2);

			setConnectionStatus('reconnecting');

			// Service data should be preserved — map not cleared
			expect(getSortedServices()).toHaveLength(2);
			expect(getCounts().total).toBe(2);
			expect(getLastUpdated()).toBeInstanceOf(Date);
		});

		it('transitions through states correctly including reconnecting', () => {
			expect(getConnectionStatus()).toBe('connecting');
			setConnectionStatus('connected');
			expect(getConnectionStatus()).toBe('connected');
			setConnectionStatus('reconnecting');
			expect(getConnectionStatus()).toBe('reconnecting');
			setConnectionStatus('connected');
			expect(getConnectionStatus()).toBe('connected');
			setConnectionStatus('disconnected');
			expect(getConnectionStatus()).toBe('disconnected');
		});
	});

	describe('groupedServices', () => {
		it('groups mixed health states correctly — unhealthy and authBlocked in needsAttention, healthy in healthy', () => {
			replaceAll([
				makeService({ name: 'good-svc', status: 'healthy' }),
				makeService({ name: 'bad-svc', status: 'unhealthy' }),
				makeService({ name: 'blocked-svc', status: 'authBlocked' })
			], 'v1.0.0');
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
			], 'v1.0.0');
			const groups = getGroupedServices();
			expect(groups.needsAttention.map((s) => s.name)).toEqual(['bravo', 'delta']);
			expect(groups.healthy.map((s) => s.name)).toEqual(['alpha', 'zebra']);
		});

		it('returns empty needsAttention when all services are healthy', () => {
			replaceAll([
				makeService({ name: 'alpha', status: 'healthy' }),
				makeService({ name: 'bravo', status: 'healthy' })
			], 'v1.0.0');
			const groups = getGroupedServices();
			expect(groups.needsAttention).toEqual([]);
			expect(groups.healthy.map((s) => s.name)).toEqual(['alpha', 'bravo']);
		});

		it('returns empty healthy when all services are unhealthy', () => {
			replaceAll([
				makeService({ name: 'bad-a', status: 'unhealthy' }),
				makeService({ name: 'bad-b', status: 'authBlocked' })
			], 'v1.0.0');
			const groups = getGroupedServices();
			expect(groups.needsAttention.map((s) => s.name)).toEqual(['bad-a', 'bad-b']);
			expect(groups.healthy).toEqual([]);
		});

		it('places unknown status services in needsAttention group', () => {
			replaceAll([
				makeService({ name: 'new-svc', status: 'unknown' }),
				makeService({ name: 'good-svc', status: 'healthy' })
			], 'v1.0.0');
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
			], 'v1.0.0');
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
			], 'v1.0.0');
			// bravo first (unhealthy), then alpha (healthy)
			expect(getGroupedServices().needsAttention.map((s) => s.name)).toEqual(['bravo']);
			expect(getGroupedServices().healthy.map((s) => s.name)).toEqual(['alpha']);

			// SSE reconnect — replaceAll with alpha now unhealthy
			replaceAll([
				makeService({ name: 'alpha', status: 'unhealthy' }),
				makeService({ name: 'bravo', status: 'healthy' })
			], 'v2.0.0');
			// Sort order recalculated: alpha (unhealthy) first, then bravo (healthy)
			expect(getGroupedServices().needsAttention.map((s) => s.name)).toEqual(['alpha']);
			expect(getGroupedServices().healthy.map((s) => s.name)).toEqual(['bravo']);
		});

		it('recalculates sort order when a new service appears via addOrUpdate', () => {
			replaceAll([makeService({ name: 'bravo', status: 'healthy' })], 'v1.0.0');
			expect(getGroupedServices().healthy.map((s) => s.name)).toEqual(['bravo']);

			// New service via addOrUpdate — triggers sort recalc since key is new
			addOrUpdate(makeService({ name: 'alpha', status: 'healthy' }));
			expect(getGroupedServices().healthy.map((s) => s.name)).toEqual(['alpha', 'bravo']);
		});
	});

	describe('k8sStatus', () => {
		it('defaults to disconnected with null lastEvent', () => {
			expect(getK8sConnected()).toBe(false);
			expect(getK8sLastEvent()).toBeNull();
		});

		it('setK8sStatus(true) updates k8sConnected', () => {
			setK8sStatus(true, '2026-02-20T14:30:00Z');
			expect(getK8sConnected()).toBe(true);
		});

		it('setK8sStatus(false) updates k8sConnected', () => {
			setK8sStatus(true, '2026-02-20T14:30:00Z');
			setK8sStatus(false, '2026-02-20T14:31:00Z');
			expect(getK8sConnected()).toBe(false);
		});

		it('parses lastEvent as Date', () => {
			setK8sStatus(true, '2026-02-20T14:30:00Z');
			const lastEvent = getK8sLastEvent();
			expect(lastEvent).toBeInstanceOf(Date);
			expect(lastEvent!.toISOString()).toBe('2026-02-20T14:30:00.000Z');
		});

		it('handles null lastEvent', () => {
			setK8sStatus(true, null);
			expect(getK8sLastEvent()).toBeNull();
		});

		it('is reset by _resetForTesting', () => {
			setK8sStatus(true, '2026-02-20T14:30:00Z');
			_resetForTesting();
			expect(getK8sConnected()).toBe(false);
			expect(getK8sLastEvent()).toBeNull();
		});
	});

	describe('healthCheckIntervalMs', () => {
		it('defaults to 30000', () => {
			expect(getHealthCheckIntervalMs()).toBe(30_000);
		});

		it('stores interval from replaceAll', () => {
			replaceAll([makeService({ name: 'svc' })], 'v1.0.0', 15_000);
			expect(getHealthCheckIntervalMs()).toBe(15_000);
		});

		it('falls back to 30000 when not provided', () => {
			replaceAll([makeService({ name: 'svc' })], 'v1.0.0');
			expect(getHealthCheckIntervalMs()).toBe(30_000);
		});

		it('is reset by _resetForTesting', () => {
			replaceAll([makeService({ name: 'svc' })], 'v1.0.0', 15_000);
			_resetForTesting();
			expect(getHealthCheckIntervalMs()).toBe(30_000);
		});
	});

	describe('lastUpdated', () => {
		it('is null initially', () => {
			expect(getLastUpdated()).toBeNull();
		});

		it('uses newest service lastChecked on replaceAll', () => {
			const older = '2026-02-20T12:00:00.000Z';
			const newer = '2026-02-20T12:01:00.000Z';
			replaceAll([
				makeService({ name: 'svc-a', lastChecked: older }),
				makeService({ name: 'svc-b', lastChecked: newer })
			], 'v1.0.0');
			expect(getLastUpdated()?.toISOString()).toBe(newer);
		});

		it('keeps lastUpdated null on replaceAll when no service has lastChecked', () => {
			replaceAll([makeService({ name: 'svc' })], 'v1.0.0');
			expect(getLastUpdated()).toBeNull();
		});

		it('updates on addOrUpdate when service includes lastChecked', () => {
			const checkedAt = '2026-02-20T12:02:00.000Z';
			addOrUpdate(makeService({ name: 'svc', lastChecked: checkedAt }));
			expect(getLastUpdated()?.toISOString()).toBe(checkedAt);
		});

		it('does not update on addOrUpdate when service has no lastChecked', () => {
			addOrUpdate(makeService({ name: 'svc' }));
			expect(getLastUpdated()).toBeNull();
		});

		it('does not change on remove', () => {
			const checkedAt = '2026-02-20T12:03:00.000Z';
			replaceAll([makeService({ name: 'svc', lastChecked: checkedAt })], 'v1.0.0');
			remove('default', 'svc');
			expect(getLastUpdated()?.toISOString()).toBe(checkedAt);
		});
	});
});
