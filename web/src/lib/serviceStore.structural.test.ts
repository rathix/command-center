import { describe, it, expect } from 'vitest';
import * as store from './serviceStore.svelte';

describe('serviceStore structural integrity', () => {
	it('_resetForTesting resets all state reachable via getters', () => {
		// 1. Capture initial values (defaults)
		const getters = Object.keys(store).filter(key => 
			key.startsWith('get') && typeof (store as Record<string, unknown>)[key] === 'function'
		);
		const defaults = new Map<string, unknown>();
		
		getters.forEach(key => {
			const getter = (store as unknown as Record<string, () => unknown>)[key];
			defaults.set(key, getter());
		});

		// 2. Mutate state via multiple entry points
		store.replaceAll([{
			name: 'svc-1',
			namespace: 'default',
			group: 'default',
			displayName: 'Service 1',
			url: 'http://svc-1',
			status: 'healthy',
			httpCode: 200,
			responseTimeMs: 50,
			lastChecked: '2026-02-20T10:00:00Z',
			lastStateChange: '2026-02-20T09:00:00Z',
			errorSnippet: null
		}], 'v9.9.9', 60000);
		
		store.addOrUpdate({
			name: 'svc-2',
			namespace: 'other',
			group: 'other',
			displayName: 'Service 2',
			url: 'http://svc-2',
			status: 'unhealthy',
			httpCode: 500,
			responseTimeMs: 500,
			lastChecked: '2026-02-20T11:00:00Z',
			lastStateChange: '2026-02-20T10:30:00Z',
			errorSnippet: 'CRITICAL FAILURE'
		});

		store.setConnectionStatus('disconnected');
		store.setK8sStatus(true, '2026-02-20T12:00:00Z');
		store.setConfigErrors(['some error']);

		// Verify state is actually changed
		expect(store.getAppVersion()).toBe('v9.9.9');
		expect(store.getConnectionStatus()).toBe('disconnected');
		expect(store.getCounts().total).toBe(2);
		expect(store.getLastUpdated()?.toISOString()).toBe('2026-02-20T11:00:00.000Z');

		// 3. Reset
		store._resetForTesting();

		// 4. Assert all getters return original defaults
		getters.forEach(key => {
			const getter = (store as unknown as Record<string, () => unknown>)[key];
			expect(getter(), `Getter ${key} should be reset to its default value`).toEqual(defaults.get(key));
		});
	});

	it('has the expected number of getters (prevents adding state without updating this test)', () => {
		const getters = Object.keys(store).filter(key => key.startsWith('get'));
		
		// This list must be updated whenever new state/getters are added to serviceStore.svelte.ts
		const expectedGetters = [
			'getSortedServices',
			'getCounts',
			'getHasProblems',
			'getConnectionStatus',
			'getLastUpdated',
			'getServiceGroups',
			'getAppVersion',
			'getK8sConnected',
			'getK8sLastEvent',
			'getHealthCheckIntervalMs',
			'getConfigErrors',
			'getHasConfigErrors'
		];

		expect(getters.sort()).toEqual(expectedGetters.sort());
	});
});
