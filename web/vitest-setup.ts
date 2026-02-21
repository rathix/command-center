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
