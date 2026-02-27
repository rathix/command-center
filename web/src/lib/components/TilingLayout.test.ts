import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import TilingLayout from './TilingLayout.svelte';
import type { LeafNode, BranchNode } from '$lib/types';

vi.mock('$lib/layoutStore.svelte', () => ({
	resizePanel: vi.fn(),
	getActivePanelId: () => 'leaf-1',
	setActivePanel: vi.fn(),
	splitPanel: vi.fn(),
	closePanel: vi.fn(),
	getIsLastPanel: () => true,
	AVAILABLE_PANEL_TYPES: new Set(['services']),
	setPanelType: vi.fn()
}));

function makeLeaf(id: string = 'leaf-1'): LeafNode {
	return { type: 'leaf', panelType: 'services', panelId: id };
}

function makeBranch(
	direction: 'horizontal' | 'vertical' = 'horizontal'
): BranchNode {
	return {
		type: 'branch',
		direction,
		ratio: 0.5,
		first: makeLeaf('leaf-1'),
		second: makeLeaf('leaf-2')
	};
}

describe('TilingLayout', () => {
	it('renders single Panel for leaf node', () => {
		render(TilingLayout, { props: { node: makeLeaf() } });
		expect(screen.getByTestId('panel')).toBeInTheDocument();
	});

	it('renders two children with gutter for branch node', () => {
		render(TilingLayout, { props: { node: makeBranch() } });
		const panels = screen.getAllByTestId('panel');
		expect(panels.length).toBe(2);
		expect(screen.getByTestId('gutter')).toBeInTheDocument();
	});

	it('horizontal branch uses flex-row', () => {
		render(TilingLayout, { props: { node: makeBranch('horizontal') } });
		const branch = screen.getByTestId('branch');
		expect(branch.className).toContain('flex-row');
	});

	it('vertical branch uses flex-col', () => {
		render(TilingLayout, { props: { node: makeBranch('vertical') } });
		const branch = screen.getByTestId('branch');
		expect(branch.className).toContain('flex-col');
	});
});
