import {
	getBindings,
	getSuppressShortcuts,
	setSuppressShortcuts,
	toggleOverlay,
	ACTIONS
} from './keyboardStore.svelte';
import {
	splitPanel,
	closePanel,
	getActivePanelId,
	focusAdjacentPanel
} from './layoutStore.svelte';

function buildKeyCombo(event: KeyboardEvent): string {
	const parts: string[] = [];
	if (event.ctrlKey) parts.push('Ctrl');
	if (event.shiftKey) parts.push('Shift');
	if (event.altKey) parts.push('Alt');
	if (event.metaKey) parts.push('Meta');
	parts.push(event.key);
	return parts.join('+');
}

export function handleKeyDown(event: KeyboardEvent): void {
	// Ctrl+Shift+Escape always handled (terminal escape hatch)
	const combo = buildKeyCombo(event);
	if (combo === 'Ctrl+Shift+Escape') {
		event.preventDefault();
		setSuppressShortcuts(false);
		return;
	}

	if (getSuppressShortcuts()) return;

	// Check plain ? key (no modifiers)
	if (event.key === '?' && !event.altKey && !event.ctrlKey && !event.metaKey) {
		event.preventDefault();
		toggleOverlay();
		return;
	}

	const bindings = getBindings();
	const action = bindings.get(combo);
	if (!action) return;

	event.preventDefault();

	const panelId = getActivePanelId();

	switch (action) {
		case ACTIONS.SPLIT_H:
			splitPanel(panelId, 'horizontal');
			break;
		case ACTIONS.SPLIT_V:
			splitPanel(panelId, 'vertical');
			break;
		case ACTIONS.CLOSE_PANEL:
			closePanel(panelId);
			break;
		case ACTIONS.FOCUS_UP:
			focusAdjacentPanel('up');
			break;
		case ACTIONS.FOCUS_DOWN:
			focusAdjacentPanel('down');
			break;
		case ACTIONS.FOCUS_LEFT:
			focusAdjacentPanel('left');
			break;
		case ACTIONS.FOCUS_RIGHT:
			focusAdjacentPanel('right');
			break;
		case ACTIONS.SHOW_HELP:
			toggleOverlay();
			break;
	}
}
