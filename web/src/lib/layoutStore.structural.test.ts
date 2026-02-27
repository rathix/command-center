import { describe, it, expect } from 'vitest';
import * as store from './layoutStore.svelte';

describe('layoutStore structural integrity', () => {
	it('has the expected number of getters', () => {
		const getters = Object.keys(store).filter((key) => key.startsWith('get'));

		const expectedGetters = [
			'getRootNode',
			'getActivePanelId',
			'getIsLastPanel',
			'getPresets',
			'getActivePresetName'
		];

		expect(getters.sort()).toEqual(expectedGetters.sort());
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

		// Mutate state
		const root = store.getRootNode();
		if (root.type === 'leaf') {
			store.splitPanel(root.panelId, 'horizontal');
		}

		// Reset
		store._resetForTesting();

		// Verify getters return defaults (some values will be different due to UUID generation)
		const resetRoot = store.getRootNode();
		expect(resetRoot.type).toBe('leaf');
		expect(store.getIsLastPanel()).toBe(true);
		expect(store.getPresets().size).toBe(0);
		expect(store.getActivePresetName()).toBe(null);
	});
});
