import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

// Mock Web Animations API for Svelte transitions in jsdom
if (typeof window !== 'undefined') {
	Element.prototype.animate = vi.fn().mockImplementation(() => ({
		finished: Promise.resolve(),
		cancel: vi.fn(),
		onfinish: null
	}));
}

// Mock matchMedia for breakpoint detection in jsdom
if (typeof window !== 'undefined' && !window.matchMedia) {
	Object.defineProperty(window, 'matchMedia', {
		writable: true,
		value: vi.fn().mockImplementation((query: string) => ({
			matches: false,
			media: query,
			onchange: null,
			addListener: vi.fn(),
			removeListener: vi.fn(),
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
			dispatchEvent: vi.fn()
		}))
	});
}
