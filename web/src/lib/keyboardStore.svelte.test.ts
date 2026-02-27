import { describe, it, expect, beforeEach } from 'vitest';
import * as store from './keyboardStore.svelte';

describe('keyboardStore', () => {
	beforeEach(() => {
		store._resetForTesting();
	});

	describe('default bindings', () => {
		it('includes Alt+h mapped to splitH', () => {
			expect(store.getBindings().get('Alt+h')).toBe('splitH');
		});

		it('includes Alt+v mapped to splitV', () => {
			expect(store.getBindings().get('Alt+v')).toBe('splitV');
		});

		it('includes Alt+q mapped to closePanel', () => {
			expect(store.getBindings().get('Alt+q')).toBe('closePanel');
		});

		it('includes Alt+Arrow keys for focus navigation', () => {
			const bindings = store.getBindings();
			expect(bindings.get('Alt+ArrowUp')).toBe('focusUp');
			expect(bindings.get('Alt+ArrowDown')).toBe('focusDown');
			expect(bindings.get('Alt+ArrowLeft')).toBe('focusLeft');
			expect(bindings.get('Alt+ArrowRight')).toBe('focusRight');
		});

		it('includes ? mapped to showHelp', () => {
			expect(store.getBindings().get('?')).toBe('showHelp');
		});
	});

	describe('loadCustomBindings', () => {
		it('overrides specific bindings', () => {
			store.loadCustomBindings({
				mod: 'Ctrl',
				bindings: { h: 'splitH' }
			});
			expect(store.getBindings().get('Ctrl+h')).toBe('splitH');
		});

		it('preserves non-overridden defaults', () => {
			store.loadCustomBindings({
				mod: 'Ctrl',
				bindings: { h: 'splitH' }
			});
			// Alt+v should still exist from defaults
			expect(store.getBindings().get('Alt+v')).toBe('splitV');
		});
	});

	describe('suppressShortcuts', () => {
		it('defaults to false', () => {
			expect(store.getSuppressShortcuts()).toBe(false);
		});

		it('setSuppressShortcuts(true) sets flag', () => {
			store.setSuppressShortcuts(true);
			expect(store.getSuppressShortcuts()).toBe(true);
		});
	});

	describe('overlay', () => {
		it('defaults to hidden', () => {
			expect(store.getShowOverlay()).toBe(false);
		});

		it('toggleOverlay shows then hides', () => {
			store.toggleOverlay();
			expect(store.getShowOverlay()).toBe(true);
			store.toggleOverlay();
			expect(store.getShowOverlay()).toBe(false);
		});

		it('hideOverlay sets to false', () => {
			store.toggleOverlay();
			store.hideOverlay();
			expect(store.getShowOverlay()).toBe(false);
		});
	});

	describe('getBindingsList', () => {
		it('returns all bindings with descriptions', () => {
			const list = store.getBindingsList();
			expect(list.length).toBeGreaterThan(0);
			const splitH = list.find((b) => b.action === 'splitH');
			expect(splitH).toBeDefined();
			expect(splitH!.description).toBe('Split panel horizontally');
		});
	});

	describe('_resetForTesting', () => {
		it('restores defaults', () => {
			store.setSuppressShortcuts(true);
			store.toggleOverlay();
			store.loadCustomBindings({ mod: 'Ctrl', bindings: { x: 'custom' } });
			store._resetForTesting();

			expect(store.getSuppressShortcuts()).toBe(false);
			expect(store.getShowOverlay()).toBe(false);
			expect(store.getBindings().get('Alt+h')).toBe('splitH');
		});
	});
});
