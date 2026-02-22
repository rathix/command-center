import { describe, it, expect, beforeEach } from 'vitest';
import {
	getSortedServices,
	getServiceGroups,
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
	toggleGroupCollapse,
	_resetForTesting
} from './serviceStore.svelte';
import type { Service } from './types';

function makeService(overrides: Partial<Service> & { name: string }): Service {
	return {
		displayName: overrides.displayName ?? overrides.name,
		namespace: 'default',
		group: 'default',
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
		it('sorts problems first: unhealthy → unknown → healthy', () => {
			replaceAll([
				makeService({ name: 'healthy-svc', status: 'healthy' }),
				makeService({ name: 'unknown-svc', status: 'unknown' }),
				makeService({ name: 'unhealthy-svc', status: 'unhealthy' })
			], 'v1.0.0');
			expect(getSortedServices().map((s) => s.status)).toEqual([
				'unhealthy',
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
				makeService({ name: 'k1', status: 'unknown' })
			], 'v1.0.0');
			expect(getCounts()).toEqual({
				total: 4,
				healthy: 2,
				unhealthy: 1,
				unknown: 1
			});
		});

		it('returns all zeros for empty store', () => {
			expect(getCounts()).toEqual({
				total: 0,
				healthy: 0,
				unhealthy: 0,
				unknown: 0
			});
		});
	});

	describe('hasProblems', () => {
		it('is true when unhealthy services exist', () => {
			replaceAll([makeService({ name: 'bad', status: 'unhealthy' })], 'v1.0.0');
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

	describe('serviceGroups', () => {
		it('returns services grouped by group field', () => {
			replaceAll([
				makeService({ name: 'svc-a', group: 'media', status: 'healthy' }),
				makeService({ name: 'svc-b', group: 'infra', status: 'healthy' }),
				makeService({ name: 'svc-c', group: 'media', status: 'healthy' })
			], 'v1.0.0');
			const groups = getServiceGroups();
			expect(groups).toHaveLength(2);
			const mediaGroup = groups.find(g => g.name === 'media');
			const infraGroup = groups.find(g => g.name === 'infra');
			expect(mediaGroup!.services).toHaveLength(2);
			expect(infraGroup!.services).toHaveLength(1);
		});

		it('computes per-group counts correctly', () => {
			replaceAll([
				makeService({ name: 'h1', group: 'media', status: 'healthy' }),
				makeService({ name: 'h2', group: 'media', status: 'healthy' }),
				makeService({ name: 'h3', group: 'media', status: 'healthy' }),
				makeService({ name: 'h4', group: 'media', status: 'healthy' }),
				makeService({ name: 'h5', group: 'media', status: 'healthy' }),
				makeService({ name: 'h6', group: 'media', status: 'healthy' }),
				makeService({ name: 'h7', group: 'media', status: 'healthy' }),
				makeService({ name: 'u1', group: 'media', status: 'unhealthy' })
			], 'v1.0.0');
			const groups = getServiceGroups();
			const media = groups.find(g => g.name === 'media')!;
			expect(media.counts).toEqual({ healthy: 7, unhealthy: 1, unknown: 0 });
		});

		it('hasProblems is true when group has unhealthy/unknown services', () => {
			replaceAll([
				makeService({ name: 'a', group: 'good', status: 'healthy' }),
				makeService({ name: 'b', group: 'good', status: 'healthy' }),
				makeService({ name: 'c', group: 'bad', status: 'healthy' }),
				makeService({ name: 'd', group: 'bad', status: 'unhealthy' }),
				makeService({ name: 'e', group: 'mixed', status: 'healthy' }),
				makeService({ name: 'f', group: 'mixed', status: 'unknown' })
			], 'v1.0.0');
			const groups = getServiceGroups();
			expect(groups.find(g => g.name === 'good')!.hasProblems).toBe(false);
			expect(groups.find(g => g.name === 'bad')!.hasProblems).toBe(true);
			expect(groups.find(g => g.name === 'mixed')!.hasProblems).toBe(true);
		});

		it('groups with problems sort before all-healthy groups', () => {
			replaceAll([
				makeService({ name: 'a', group: 'alpha', status: 'healthy' }),
				makeService({ name: 'b', group: 'beta', status: 'unhealthy' }),
				makeService({ name: 'c', group: 'gamma', status: 'healthy' })
			], 'v1.0.0');
			const groups = getServiceGroups();
			expect(groups.map(g => g.name)).toEqual(['beta', 'alpha', 'gamma']);
		});

		it('group sorting is 3-tier: unhealthy first, then other problem groups, then healthy alphabetical', () => {
			replaceAll([
				makeService({ name: 'a', group: 'alpha', status: 'healthy' }),
				makeService({ name: 'c', group: 'gamma', status: 'unhealthy' }),
				makeService({ name: 'd', group: 'delta', status: 'unknown' }),
				makeService({ name: 'e', group: 'epsilon', status: 'healthy' })
			], 'v1.0.0');

			const groups = getServiceGroups();
			expect(groups.map(g => g.name)).toEqual(['gamma', 'delta', 'alpha', 'epsilon']);
		});

		it('groups with more unhealthy services sort higher', () => {
			replaceAll([
				makeService({ name: 'a', group: 'group-1', status: 'unhealthy' }),
				makeService({ name: 'b', group: 'group-2', status: 'unhealthy' }),
				makeService({ name: 'c', group: 'group-2', status: 'unhealthy' }),
				makeService({ name: 'd', group: 'group-3', status: 'healthy' })
			], 'v1.0.0');
			const groups = getServiceGroups();
			// group-2 (2 unhealthy) > group-1 (1 unhealthy) > group-3 (0 unhealthy)
			expect(groups.map(g => g.name)).toEqual(['group-2', 'group-1', 'group-3']);
		});

		it('within-group sorting follows status priority then alphabetical', () => {
			replaceAll([
				makeService({ name: 'zebra', group: 'ns', status: 'healthy' }),
				makeService({ name: 'alpha', group: 'ns', status: 'healthy' }),
				makeService({ name: 'delta', group: 'ns', status: 'unknown' }),
				makeService({ name: 'echo', group: 'ns', status: 'unhealthy' })
			], 'v1.0.0');
			const group = getServiceGroups().find(g => g.name === 'ns')!;
			expect(group.services.map(s => s.name)).toEqual([
				'echo', 'delta', 'alpha', 'zebra'
			]);
		});

		it('all-healthy groups sort alphabetically among themselves', () => {
			replaceAll([
				makeService({ name: 'a', group: 'zebra', status: 'healthy' }),
				makeService({ name: 'b', group: 'alpha', status: 'healthy' }),
				makeService({ name: 'c', group: 'mid', status: 'healthy' })
			], 'v1.0.0');
			const groups = getServiceGroups();
			expect(groups.map(g => g.name)).toEqual(['alpha', 'mid', 'zebra']);
		});

		it('empty store returns empty array', () => {
			expect(getServiceGroups()).toEqual([]);
		});

		it('toggleGroupCollapse flips expanded state and _resetForTesting clears overrides', () => {
			replaceAll([
				makeService({ name: 'a', group: 'healthy-group', status: 'healthy' }),
				makeService({ name: 'b', group: 'problem-group', status: 'unhealthy' })
			], 'v1.0.0');

			// Default: healthy-group collapsed (expanded=false), problem-group expanded (expanded=true)
			let groups = getServiceGroups();
			expect(groups.find(g => g.name === 'healthy-group')!.expanded).toBe(false);
			expect(groups.find(g => g.name === 'problem-group')!.expanded).toBe(true);

			// Toggle healthy-group → should become expanded
			toggleGroupCollapse('healthy-group');
			groups = getServiceGroups();
			expect(groups.find(g => g.name === 'healthy-group')!.expanded).toBe(true);

			// Toggle problem-group → should become collapsed
			toggleGroupCollapse('problem-group');
			groups = getServiceGroups();
			expect(groups.find(g => g.name === 'problem-group')!.expanded).toBe(false);

			// Reset clears overrides — back to defaults
			_resetForTesting();
			replaceAll([
				makeService({ name: 'a', group: 'healthy-group', status: 'healthy' }),
				makeService({ name: 'b', group: 'problem-group', status: 'unhealthy' })
			], 'v1.0.0');
			groups = getServiceGroups();
			expect(groups.find(g => g.name === 'healthy-group')!.expanded).toBe(false);
			expect(groups.find(g => g.name === 'problem-group')!.expanded).toBe(true);
		});

		it('prunes stale collapse overrides when groups disappear', () => {
			replaceAll([
				makeService({ name: 'a', group: 'healthy-group', status: 'healthy' }),
				makeService({ name: 'b', group: 'temp-group', status: 'unhealthy' })
			], 'v1.0.0');

			// temp-group starts expanded (problem group), toggle to collapsed
			toggleGroupCollapse('temp-group');
			expect(getServiceGroups().find(g => g.name === 'temp-group')!.expanded).toBe(false);

			// Remove the only service in temp-group, then re-add the group later.
			// The old override should be discarded.
			remove('default', 'b');
			addOrUpdate(makeService({ name: 'c', group: 'temp-group', status: 'unhealthy' }));

			expect(getServiceGroups().find(g => g.name === 'temp-group')!.expanded).toBe(true);
		});
	});

	describe('pre-populated startup state', () => {
		it('replaceAll preserves lastStateChange and status when lastChecked is null', () => {
			replaceAll([
				makeService({
					name: 'svc-1',
					status: 'healthy',
					lastChecked: null,
					lastStateChange: '2026-02-18T10:00:00Z'
				})
			], 'v1.0.0');
			const services = getSortedServices();
			expect(services).toHaveLength(1);
			expect(services[0].status).toBe('healthy');
			expect(services[0].lastStateChange).toBe('2026-02-18T10:00:00Z');
			expect(services[0].lastChecked).toBeNull();
		});

		it('counts are correct when services have null lastChecked', () => {
			replaceAll([
				makeService({ name: 'svc-1', status: 'healthy', lastChecked: null, lastStateChange: '2026-02-18T10:00:00Z' }),
				makeService({ name: 'svc-2', status: 'unhealthy', lastChecked: null, lastStateChange: '2026-02-18T10:00:00Z' })
			], 'v1.0.0');
			expect(getCounts()).toEqual({
				total: 2,
				healthy: 1,
				unhealthy: 1,
				unknown: 0
			});
		});

		it('lastUpdated is null when all services have null lastChecked', () => {
			replaceAll([
				makeService({ name: 'svc-1', status: 'healthy', lastChecked: null, lastStateChange: '2026-02-18T10:00:00Z' })
			], 'v1.0.0');
			expect(getLastUpdated()).toBeNull();
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
