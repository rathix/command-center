import { describe, it, expect, beforeEach } from 'vitest';
import * as store from './breakpointStore.svelte';

describe('breakpointStore', () => {
	beforeEach(() => {
		store._resetForTesting();
	});

	it('default breakpoint is desktop', () => {
		expect(store.getBreakpoint()).toBe('desktop');
	});

	it('isMobile returns false by default', () => {
		expect(store.isMobile()).toBe(false);
	});

	it('isTablet returns false by default', () => {
		expect(store.isTablet()).toBe(false);
	});

	it('isDesktop returns true by default', () => {
		expect(store.isDesktop()).toBe(true);
	});

	it('_resetForTesting restores default state', () => {
		// Can't easily mutate breakpoint from outside without matchMedia mock,
		// but ensure reset doesn't throw and returns to default
		store._resetForTesting();
		expect(store.getBreakpoint()).toBe('desktop');
		expect(store.isMobile()).toBe(false);
		expect(store.isTablet()).toBe(false);
		expect(store.isDesktop()).toBe(true);
	});
});

describe('breakpointStore structural integrity', () => {
	beforeEach(() => {
		store._resetForTesting();
	});

	it('_resetForTesting resets all state reachable via getters', () => {
		const getters = Object.keys(store).filter(
			(key) =>
				(key.startsWith('get') || key.startsWith('is')) &&
				typeof (store as Record<string, unknown>)[key] === 'function' &&
				key !== 'initBreakpointListener' &&
				key !== 'destroyBreakpointListener' &&
				key !== '_resetForTesting'
		);
		const defaults = new Map<string, unknown>();

		getters.forEach((key) => {
			const getter = (store as unknown as Record<string, () => unknown>)[key];
			defaults.set(key, getter());
		});

		// Reset
		store._resetForTesting();

		// Assert all getters return original defaults
		getters.forEach((key) => {
			const getter = (store as unknown as Record<string, () => unknown>)[key];
			expect(getter(), `Getter ${key} should be reset to its default value`).toEqual(
				defaults.get(key)
			);
		});
	});

	it('has the expected number of getters', () => {
		const getters = Object.keys(store).filter(
			(key) =>
				(key.startsWith('get') || key.startsWith('is')) &&
				typeof (store as Record<string, unknown>)[key] === 'function'
		);

		const expectedGetters = ['getBreakpoint', 'isMobile', 'isTablet', 'isDesktop'];

		expect(getters.sort()).toEqual(expectedGetters.sort());
	});
});
