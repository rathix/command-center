import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import Panel from './Panel.svelte';
import type { LeafNode } from '$lib/types';

const mockSetActivePanel = vi.fn();
const mockSplitPanel = vi.fn();
const mockClosePanel = vi.fn();
const mockSetPanelType = vi.fn();
let mockActivePanelId = 'test-id';
let mockIsLastPanel = false;

vi.mock('$lib/layoutStore.svelte', () => ({
	getActivePanelId: () => mockActivePanelId,
	setActivePanel: (...args: unknown[]) => mockSetActivePanel(...args),
	splitPanel: (...args: unknown[]) => mockSplitPanel(...args),
	closePanel: (...args: unknown[]) => mockClosePanel(...args),
	getIsLastPanel: () => mockIsLastPanel,
	AVAILABLE_PANEL_TYPES: new Set(['services']),
	setPanelType: (...args: unknown[]) => mockSetPanelType(...args)
}));

function makeLeaf(overrides: Partial<LeafNode> = {}): LeafNode {
	return {
		type: 'leaf',
		panelType: 'services',
		panelId: 'test-id',
		...overrides
	};
}

describe('Panel', () => {
	beforeEach(() => {
		mockSetActivePanel.mockClear();
		mockSplitPanel.mockClear();
		mockClosePanel.mockClear();
		mockSetPanelType.mockClear();
		mockActivePanelId = 'test-id';
		mockIsLastPanel = false;
	});

	it('renders header with "Services" label for services panel', () => {
		render(Panel, { props: { node: makeLeaf() } });
		expect(screen.getByText('Services')).toBeInTheDocument();
	});

	it('renders header with capitalized panel type', () => {
		render(Panel, { props: { node: makeLeaf({ panelType: 'terminal' }) } });
		const header = screen.getByTestId('panel-header');
		expect(header.textContent).toContain('Terminal');
	});

	it('clicking panel calls setActivePanel', async () => {
		render(Panel, { props: { node: makeLeaf({ panelId: 'click-test' }) } });
		const panel = screen.getByTestId('panel');
		await fireEvent.click(panel);
		expect(mockSetActivePanel).toHaveBeenCalledWith('click-test');
	});

	it('active panel has active border class', () => {
		mockActivePanelId = 'test-id';
		render(Panel, { props: { node: makeLeaf({ panelId: 'test-id' }) } });
		const panel = screen.getByTestId('panel');
		expect(panel.className).toContain('border-[var(--color-panel-active)]');
	});

	it('inactive panel has panel-border class', () => {
		mockActivePanelId = 'other-id';
		render(Panel, { props: { node: makeLeaf({ panelId: 'test-id' }) } });
		const panel = screen.getByTestId('panel');
		expect(panel.className).toContain('border-[var(--color-panel-border)]');
	});

	it('split button opens dropdown with H/V options', async () => {
		render(Panel, { props: { node: makeLeaf() } });
		const splitBtn = screen.getByTestId('split-button');
		await fireEvent.click(splitBtn);
		expect(screen.getByTestId('split-horizontal')).toBeInTheDocument();
		expect(screen.getByTestId('split-vertical')).toBeInTheDocument();
	});

	it('selecting "Split Horizontal" calls splitPanel', async () => {
		render(Panel, { props: { node: makeLeaf() } });
		await fireEvent.click(screen.getByTestId('split-button'));
		await fireEvent.click(screen.getByTestId('split-horizontal'));
		expect(mockSplitPanel).toHaveBeenCalledWith('test-id', 'horizontal');
	});

	it('selecting "Split Vertical" calls splitPanel', async () => {
		render(Panel, { props: { node: makeLeaf() } });
		await fireEvent.click(screen.getByTestId('split-button'));
		await fireEvent.click(screen.getByTestId('split-vertical'));
		expect(mockSplitPanel).toHaveBeenCalledWith('test-id', 'vertical');
	});

	it('close button calls closePanel', async () => {
		render(Panel, { props: { node: makeLeaf() } });
		const closeBtn = screen.getByTestId('close-button');
		await fireEvent.click(closeBtn);
		expect(mockClosePanel).toHaveBeenCalledWith('test-id');
	});

	it('close button hidden when isLastPanel is true', () => {
		mockIsLastPanel = true;
		render(Panel, { props: { node: makeLeaf() } });
		expect(screen.queryByTestId('close-button')).not.toBeInTheDocument();
	});

	it('shows unavailable label for non-available panel types', () => {
		render(Panel, { props: { node: makeLeaf({ panelType: 'terminal' }) } });
		expect(screen.getByText('(unavailable)')).toBeInTheDocument();
	});
});
