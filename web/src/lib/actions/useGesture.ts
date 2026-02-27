import { createGestureHandler, type GestureCallbacks } from '$lib/touchGestures';
import type { Action } from 'svelte/action';

export const gesture: Action<HTMLElement, GestureCallbacks> = (node, params) => {
	let cleanup = createGestureHandler(node, params);

	return {
		update(newParams: GestureCallbacks) {
			cleanup();
			cleanup = createGestureHandler(node, newParams);
		},
		destroy() {
			cleanup();
		}
	};
};
