import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import Page from './+page.svelte';

// Mock EventSource for jsdom environment
beforeEach(() => {
	globalThis.EventSource = class {
		addEventListener = vi.fn();
		close = vi.fn();
		onopen: (() => void) | null = null;
		onerror: (() => void) | null = null;
	} as unknown as typeof EventSource;
});

describe('+page.svelte', () => {
	it('renders a skip-to-content link', () => {
		render(Page);
		const skipLink = screen.getByText('Skip to service list');
		expect(skipLink).toBeInTheDocument();
		expect(skipLink).toHaveAttribute('href', '#service-list');
	});

	it('renders a StatusBar placeholder area with banner role', () => {
		render(Page);
		const statusBar = screen.getByRole('banner');
		expect(statusBar).toBeInTheDocument();
		expect(statusBar).toHaveClass('fixed');
	});

	it('renders a main content area with service-list id', () => {
		render(Page);
		const main = screen.getByRole('main');
		expect(main).toBeInTheDocument();
		expect(main).toHaveAttribute('id', 'service-list');
		expect(main).toHaveAttribute('tabindex', '-1');
	});
});
