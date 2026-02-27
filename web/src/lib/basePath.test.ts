import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { getBasePath, resolveApiUrl } from './basePath';

describe('basePath', () => {
	let existingBase: HTMLBaseElement | null;

	beforeEach(() => {
		existingBase = document.querySelector('base');
		// Remove any existing base tag
		if (existingBase) existingBase.remove();
	});

	afterEach(() => {
		// Clean up test base tags
		const base = document.querySelector('base');
		if (base) base.remove();
		// Restore original if any
		if (existingBase) document.head.appendChild(existingBase);
	});

	it('getBasePath returns / when no base tag present', () => {
		expect(getBasePath()).toBe('/');
	});

	it('getBasePath returns href from base tag', () => {
		const base = document.createElement('base');
		base.setAttribute('href', '/command-center/');
		document.head.appendChild(base);

		expect(getBasePath()).toBe('/command-center/');
	});

	it('resolveApiUrl constructs correct URL with default base', () => {
		expect(resolveApiUrl('api/events')).toBe('/api/events');
	});

	it('resolveApiUrl constructs correct URL with custom base', () => {
		const base = document.createElement('base');
		base.setAttribute('href', '/command-center/');
		document.head.appendChild(base);

		expect(resolveApiUrl('api/events')).toBe('/command-center/api/events');
	});

	it('resolveApiUrl handles leading slash in path', () => {
		expect(resolveApiUrl('/api/events')).toBe('/api/events');
	});

	it('resolveApiUrl handles leading slash in path with custom base', () => {
		const base = document.createElement('base');
		base.setAttribute('href', '/command-center/');
		document.head.appendChild(base);

		expect(resolveApiUrl('/api/events')).toBe('/command-center/api/events');
	});
});
