import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import PresetSelector from './PresetSelector.svelte';
import type { LayoutNode } from '$lib/types';

const mockSavePreset = vi.fn();
const mockRestorePreset = vi.fn();
const mockDeletePreset = vi.fn();
let mockActivePresetName: string | null = 'Monitoring';
const mockPresets = new Map<string, LayoutNode>([
	[
		'Monitoring',
		{ type: 'leaf', panelType: 'services', panelId: 'p1' }
	],
	[
		'Debug',
		{ type: 'leaf', panelType: 'services', panelId: 'p2' }
	]
]);

vi.mock('$lib/layoutStore.svelte', () => ({
	getPresets: () => mockPresets,
	getActivePresetName: () => mockActivePresetName,
	savePreset: (...args: unknown[]) => mockSavePreset(...args),
	restorePreset: (...args: unknown[]) => mockRestorePreset(...args),
	deletePreset: (...args: unknown[]) => mockDeletePreset(...args)
}));

describe('PresetSelector', () => {
	beforeEach(() => {
		mockSavePreset.mockClear();
		mockRestorePreset.mockClear();
		mockDeletePreset.mockClear();
		mockActivePresetName = 'Monitoring';
	});

	it('renders dropdown toggle showing active preset name', () => {
		render(PresetSelector);
		expect(screen.getByTestId('preset-toggle').textContent).toContain('Monitoring');
	});

	it('renders dropdown with preset names when opened', async () => {
		render(PresetSelector);
		await fireEvent.click(screen.getByTestId('preset-toggle'));
		expect(screen.getByTestId('preset-item-Monitoring')).toBeInTheDocument();
		expect(screen.getByTestId('preset-item-Debug')).toBeInTheDocument();
	});

	it('clicking preset calls restorePreset', async () => {
		render(PresetSelector);
		await fireEvent.click(screen.getByTestId('preset-toggle'));
		await fireEvent.click(screen.getByTestId('preset-item-Debug'));
		expect(mockRestorePreset).toHaveBeenCalledWith('Debug');
	});

	it('clicking delete calls deletePreset', async () => {
		render(PresetSelector);
		await fireEvent.click(screen.getByTestId('preset-toggle'));
		await fireEvent.click(screen.getByTestId('preset-delete-Debug'));
		expect(mockDeletePreset).toHaveBeenCalledWith('Debug');
	});

	it('"Save Layout" button opens name input', async () => {
		render(PresetSelector);
		await fireEvent.click(screen.getByTestId('preset-toggle'));
		await fireEvent.click(screen.getByTestId('preset-save-button'));
		expect(screen.getByTestId('preset-name-input')).toBeInTheDocument();
	});

	it('submitting name calls savePreset', async () => {
		render(PresetSelector);
		await fireEvent.click(screen.getByTestId('preset-toggle'));
		await fireEvent.click(screen.getByTestId('preset-save-button'));
		const input = screen.getByTestId('preset-name-input');
		await fireEvent.input(input, { target: { value: 'My Layout' } });
		await fireEvent.click(screen.getByTestId('preset-save-confirm'));
		expect(mockSavePreset).toHaveBeenCalledWith('My Layout');
	});

	it('shows "Custom" when no active preset', () => {
		mockActivePresetName = null;
		render(PresetSelector);
		expect(screen.getByTestId('preset-toggle').textContent).toContain('Custom');
	});
});
