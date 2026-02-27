import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import ContentSelector from './ContentSelector.svelte';

vi.mock('$lib/layoutStore.svelte', () => ({
	setPanelType: vi.fn(),
	AVAILABLE_PANEL_TYPES: new Set(['services'])
}));

import { setPanelType } from '$lib/layoutStore.svelte';

describe('ContentSelector', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('renders "Services" as selectable option', () => {
		render(ContentSelector, { props: { panelId: 'test-panel' } });
		const btn = screen.getByTestId('content-option-services');
		expect(btn).toBeInTheDocument();
		expect(btn).not.toBeDisabled();
	});

	it('renders disabled options for unimplemented types', () => {
		render(ContentSelector, { props: { panelId: 'test-panel' } });
		expect(screen.getByTestId('content-option-terminal')).toBeDisabled();
		expect(screen.getByTestId('content-option-logs')).toBeDisabled();
		expect(screen.getByTestId('content-option-nodes')).toBeDisabled();
		expect(screen.getByTestId('content-option-gitops')).toBeDisabled();
	});

	it('clicking "Services" calls setPanelType with correct args', async () => {
		render(ContentSelector, { props: { panelId: 'test-panel' } });
		await fireEvent.click(screen.getByTestId('content-option-services'));
		expect(setPanelType).toHaveBeenCalledWith('test-panel', 'services');
	});

	it('shows "coming soon" for unavailable types', () => {
		render(ContentSelector, { props: { panelId: 'test-panel' } });
		const comingSoon = screen.getAllByText('coming soon');
		expect(comingSoon.length).toBe(4); // terminal, logs, nodes, gitops
	});
});
