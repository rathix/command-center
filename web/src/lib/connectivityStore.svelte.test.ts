import { describe, it, expect, beforeEach } from 'vitest';
import * as store from './connectivityStore.svelte';

describe('connectivityStore', () => {
	beforeEach(() => {
		store._resetForTesting();
	});

	it('default isOnline matches navigator.onLine', () => {
		expect(store.getIsOnline()).toBe(navigator.onLine);
	});

	it('default lastSyncTime is null', () => {
		expect(store.getLastSyncTime()).toBeNull();
	});

	it('setLastSyncTime updates the value', () => {
		store.setLastSyncTime('2026-02-27T10:00:00Z');
		expect(store.getLastSyncTime()).toBe('2026-02-27T10:00:00Z');
	});

	it('_resetForTesting restores defaults', () => {
		store.setLastSyncTime('2026-02-27T10:00:00Z');
		store._resetForTesting();
		expect(store.getLastSyncTime()).toBeNull();
		expect(store.getIsOnline()).toBe(navigator.onLine);
	});
});

describe('connectivityStore structural integrity', () => {
	beforeEach(() => {
		store._resetForTesting();
	});

	it('_resetForTesting resets all state reachable via getters', () => {
		const getters = Object.keys(store).filter(
			(key) =>
				key.startsWith('get') &&
				typeof (store as Record<string, unknown>)[key] === 'function'
		);
		const defaults = new Map<string, unknown>();

		getters.forEach((key) => {
			const getter = (store as unknown as Record<string, () => unknown>)[key];
			defaults.set(key, getter());
		});

		// Mutate
		store.setLastSyncTime('2026-02-27T12:00:00Z');

		// Reset
		store._resetForTesting();

		// Assert
		getters.forEach((key) => {
			const getter = (store as unknown as Record<string, () => unknown>)[key];
			expect(getter(), `Getter ${key} should be reset`).toEqual(defaults.get(key));
		});
	});

	it('has the expected number of getters', () => {
		const getters = Object.keys(store).filter(
			(key) =>
				key.startsWith('get') &&
				typeof (store as Record<string, unknown>)[key] === 'function'
		);

		const expectedGetters = ['getIsOnline', 'getLastSyncTime'];
		expect(getters.sort()).toEqual(expectedGetters.sort());
	});
});
