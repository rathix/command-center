import type { KeyboardConfig } from './types';

// Action constants
export const ACTIONS = {
	SPLIT_H: 'splitH',
	SPLIT_V: 'splitV',
	CLOSE_PANEL: 'closePanel',
	FOCUS_UP: 'focusUp',
	FOCUS_DOWN: 'focusDown',
	FOCUS_LEFT: 'focusLeft',
	FOCUS_RIGHT: 'focusRight',
	SHOW_HELP: 'showHelp'
} as const;

const ACTION_DESCRIPTIONS: Record<string, string> = {
	splitH: 'Split panel horizontally',
	splitV: 'Split panel vertically',
	closePanel: 'Close active panel',
	focusUp: 'Focus panel above',
	focusDown: 'Focus panel below',
	focusLeft: 'Focus panel to left',
	focusRight: 'Focus panel to right',
	showHelp: 'Show keyboard shortcuts'
};

function createDefaultBindings(): Map<string, string> {
	return new Map([
		['Alt+h', ACTIONS.SPLIT_H],
		['Alt+v', ACTIONS.SPLIT_V],
		['Alt+q', ACTIONS.CLOSE_PANEL],
		['Alt+ArrowUp', ACTIONS.FOCUS_UP],
		['Alt+ArrowDown', ACTIONS.FOCUS_DOWN],
		['Alt+ArrowLeft', ACTIONS.FOCUS_LEFT],
		['Alt+ArrowRight', ACTIONS.FOCUS_RIGHT],
		['?', ACTIONS.SHOW_HELP]
	]);
}

// Internal reactive state
let bindings = $state(createDefaultBindings());
let suppressShortcuts = $state(false);
let showOverlay = $state(false);

// Exported getters
export function getBindings(): Map<string, string> {
	return bindings;
}

export function getSuppressShortcuts(): boolean {
	return suppressShortcuts;
}

export function getShowOverlay(): boolean {
	return showOverlay;
}

export function getBindingsList(): { action: string; keyCombo: string; description: string }[] {
	const result: { action: string; keyCombo: string; description: string }[] = [];
	for (const [keyCombo, action] of bindings) {
		result.push({
			action,
			keyCombo,
			description: ACTION_DESCRIPTIONS[action] ?? action
		});
	}
	return result;
}

// Mutations
export function loadCustomBindings(config: KeyboardConfig): void {
	const newBindings = createDefaultBindings();
	const mod = config.mod || 'Alt';
	for (const [key, action] of Object.entries(config.bindings)) {
		const combo = key.includes('+') ? key : `${mod}+${key}`;
		newBindings.set(combo, action);
	}
	bindings = newBindings;
}

export function setSuppressShortcuts(suppress: boolean): void {
	suppressShortcuts = suppress;
}

export function toggleOverlay(): void {
	showOverlay = !showOverlay;
}

export function hideOverlay(): void {
	showOverlay = false;
}

// Test helper
export function _resetForTesting(): void {
	bindings = createDefaultBindings();
	suppressShortcuts = false;
	showOverlay = false;
}
