import { describe, it, expect, vi, beforeEach } from 'vitest';
import { handleKeyDown } from './keyboardHandler';

const mockSplitPanel = vi.fn();
const mockClosePanel = vi.fn();
const mockFocusAdjacentPanel = vi.fn();
let mockActivePanelId = 'panel-1';

vi.mock('./layoutStore.svelte', () => ({
	splitPanel: (...args: unknown[]) => mockSplitPanel(...args),
	closePanel: (...args: unknown[]) => mockClosePanel(...args),
	getActivePanelId: () => mockActivePanelId,
	focusAdjacentPanel: (...args: unknown[]) => mockFocusAdjacentPanel(...args)
}));

const mockToggleOverlay = vi.fn();
let mockSuppressShortcuts = false;
const mockSetSuppressShortcuts = vi.fn();

vi.mock('./keyboardStore.svelte', () => ({
	getBindings: () =>
		new Map([
			['Alt+h', 'splitH'],
			['Alt+v', 'splitV'],
			['Alt+q', 'closePanel'],
			['Alt+ArrowRight', 'focusRight'],
			['Alt+ArrowLeft', 'focusLeft'],
			['Alt+ArrowUp', 'focusUp'],
			['Alt+ArrowDown', 'focusDown']
		]),
	getSuppressShortcuts: () => mockSuppressShortcuts,
	setSuppressShortcuts: (...args: unknown[]) => mockSetSuppressShortcuts(...args),
	toggleOverlay: () => mockToggleOverlay(),
	ACTIONS: {
		SPLIT_H: 'splitH',
		SPLIT_V: 'splitV',
		CLOSE_PANEL: 'closePanel',
		FOCUS_UP: 'focusUp',
		FOCUS_DOWN: 'focusDown',
		FOCUS_LEFT: 'focusLeft',
		FOCUS_RIGHT: 'focusRight',
		SHOW_HELP: 'showHelp'
	}
}));

function makeKeyEvent(
	key: string,
	opts: Partial<KeyboardEventInit> = {}
): KeyboardEvent {
	return new KeyboardEvent('keydown', {
		key,
		bubbles: true,
		cancelable: true,
		...opts
	});
}

describe('keyboardHandler', () => {
	beforeEach(() => {
		mockSplitPanel.mockClear();
		mockClosePanel.mockClear();
		mockFocusAdjacentPanel.mockClear();
		mockToggleOverlay.mockClear();
		mockSetSuppressShortcuts.mockClear();
		mockActivePanelId = 'panel-1';
		mockSuppressShortcuts = false;
	});

	it('Alt+h calls splitPanel with horizontal', () => {
		const event = makeKeyEvent('h', { altKey: true });
		handleKeyDown(event);
		expect(mockSplitPanel).toHaveBeenCalledWith('panel-1', 'horizontal');
		expect(event.defaultPrevented).toBe(true);
	});

	it('Alt+v calls splitPanel with vertical', () => {
		const event = makeKeyEvent('v', { altKey: true });
		handleKeyDown(event);
		expect(mockSplitPanel).toHaveBeenCalledWith('panel-1', 'vertical');
	});

	it('Alt+q calls closePanel', () => {
		const event = makeKeyEvent('q', { altKey: true });
		handleKeyDown(event);
		expect(mockClosePanel).toHaveBeenCalledWith('panel-1');
	});

	it('Alt+ArrowRight calls focusAdjacentPanel right', () => {
		const event = makeKeyEvent('ArrowRight', { altKey: true });
		handleKeyDown(event);
		expect(mockFocusAdjacentPanel).toHaveBeenCalledWith('right');
	});

	it('suppressed state ignores Alt+h', () => {
		mockSuppressShortcuts = true;
		const event = makeKeyEvent('h', { altKey: true });
		handleKeyDown(event);
		expect(mockSplitPanel).not.toHaveBeenCalled();
	});

	it('Ctrl+Shift+Escape always handled even when suppressed', () => {
		mockSuppressShortcuts = true;
		const event = makeKeyEvent('Escape', {
			ctrlKey: true,
			shiftKey: true
		});
		handleKeyDown(event);
		expect(mockSetSuppressShortcuts).toHaveBeenCalledWith(false);
		expect(event.defaultPrevented).toBe(true);
	});

	it('unrecognized key combo is ignored', () => {
		const event = makeKeyEvent('z', { altKey: true });
		handleKeyDown(event);
		expect(mockSplitPanel).not.toHaveBeenCalled();
		expect(mockClosePanel).not.toHaveBeenCalled();
		expect(event.defaultPrevented).toBe(false);
	});

	it('? key toggles overlay', () => {
		const event = makeKeyEvent('?');
		handleKeyDown(event);
		expect(mockToggleOverlay).toHaveBeenCalled();
	});
});
