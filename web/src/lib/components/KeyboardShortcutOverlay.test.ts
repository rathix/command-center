import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import KeyboardShortcutOverlay from './KeyboardShortcutOverlay.svelte';

vi.mock('$lib/keyboardStore.svelte', () => ({
	getBindingsList: () => [
		{ action: 'splitH', keyCombo: 'Alt+h', description: 'Split panel horizontally' },
		{ action: 'splitV', keyCombo: 'Alt+v', description: 'Split panel vertically' },
		{ action: 'closePanel', keyCombo: 'Alt+q', description: 'Close active panel' }
	]
}));

describe('KeyboardShortcutOverlay', () => {
	const onclose = vi.fn();

	beforeEach(() => {
		onclose.mockClear();
	});

	it('renders all bindings when visible', () => {
		render(KeyboardShortcutOverlay, {
			props: { visible: true, onclose }
		});
		expect(screen.getByText('Alt+h')).toBeInTheDocument();
		expect(screen.getByText('Alt+v')).toBeInTheDocument();
		expect(screen.getByText('Alt+q')).toBeInTheDocument();
		expect(screen.getByText('Split panel horizontally')).toBeInTheDocument();
	});

	it('not rendered when visible is false', () => {
		render(KeyboardShortcutOverlay, {
			props: { visible: false, onclose }
		});
		expect(screen.queryByTestId('keyboard-overlay')).not.toBeInTheDocument();
	});

	it('close button calls onclose', async () => {
		render(KeyboardShortcutOverlay, {
			props: { visible: true, onclose }
		});
		await fireEvent.click(screen.getByTestId('overlay-close'));
		expect(onclose).toHaveBeenCalled();
	});

	it('Escape key calls onclose', async () => {
		render(KeyboardShortcutOverlay, {
			props: { visible: true, onclose }
		});
		await fireEvent.keyDown(screen.getByTestId('keyboard-overlay'), {
			key: 'Escape'
		});
		expect(onclose).toHaveBeenCalled();
	});

	it('displays key combos in kbd elements', () => {
		render(KeyboardShortcutOverlay, {
			props: { visible: true, onclose }
		});
		const kbds = screen.getAllByTestId('kbd');
		expect(kbds.length).toBe(3);
	});
});
