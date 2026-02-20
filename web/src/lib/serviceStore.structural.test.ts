import { describe, it, expect } from 'vitest';
import * as store from './serviceStore.svelte';

describe('serviceStore structural integrity', () => {
	it('_resetForTesting resets all state reachable via getters', () => {
		// 1. Capture initial values (defaults)
		const getters = Object.keys(store).filter(key => key.startsWith('get'));
		const defaults = new Map<string, any>();
		
		getters.forEach(key => {
			const getter = (store as any)[key];
			defaults.set(key, getter());
		});

		// 2. Mutate state
		// We use replaceAll and other setters to change everything
		store.replaceAll([{
			name: 'test',
			namespace: 'default',
			displayName: 'Test',
			url: 'http://test',
			status: 'unhealthy',
			httpCode: 500,
			responseTimeMs: 100,
			lastChecked: new Date().toISOString(),
			lastStateChange: null,
			errorSnippet: 'error'
		}], 'v9.9.9', 60000);
		
		store.setConnectionStatus('disconnected');
		store.setK8sStatus(true, new Date().toISOString());

		// Verify state is actually changed
		expect(store.getAppVersion()).toBe('v9.9.9');
		expect(store.getConnectionStatus()).toBe('disconnected');

		// 3. Reset
		store._resetForTesting();

		// 4. Assert all getters return original defaults
		getters.forEach(key => {
			const getter = (store as any)[key];
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
			'getGroupedServices',
			'getAppVersion',
			'getK8sConnected',
			'getK8sLastEvent',
			'getHealthCheckIntervalMs'
		];

		expect(getters.sort()).toEqual(expectedGetters.sort());
	});
});
