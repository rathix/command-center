import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createGestureHandler } from './touchGestures';

function createTouchEvent(
	type: string,
	clientX: number,
	clientY: number
): TouchEvent {
	const touch = { clientX, clientY, identifier: 0 } as Touch;
	return new TouchEvent(type, {
		touches: type === 'touchend' || type === 'touchcancel' ? [] : [touch],
		changedTouches: [touch],
		cancelable: true
	});
}

describe('touchGestures', () => {
	let element: HTMLDivElement;

	beforeEach(() => {
		vi.useFakeTimers();
		element = document.createElement('div');
		document.body.appendChild(element);
	});

	afterEach(() => {
		vi.useRealTimers();
		element.remove();
	});

	it('detects tap — touchstart + touchend within 300ms, movement < 10px', () => {
		const onTap = vi.fn();
		const cleanup = createGestureHandler(element, { onTap });

		element.dispatchEvent(createTouchEvent('touchstart', 100, 100));
		vi.advanceTimersByTime(100);
		element.dispatchEvent(createTouchEvent('touchend', 102, 103));

		expect(onTap).toHaveBeenCalledOnce();
		cleanup();
	});

	it('does not detect tap when movement exceeds threshold', () => {
		const onTap = vi.fn();
		const cleanup = createGestureHandler(element, { onTap });

		element.dispatchEvent(createTouchEvent('touchstart', 100, 100));
		vi.advanceTimersByTime(100);
		element.dispatchEvent(createTouchEvent('touchend', 120, 100));

		expect(onTap).not.toHaveBeenCalled();
		cleanup();
	});

	it('detects long-press — touchstart held 500ms without movement', () => {
		const onLongPress = vi.fn();
		const cleanup = createGestureHandler(element, { onLongPress });

		element.dispatchEvent(createTouchEvent('touchstart', 100, 100));
		vi.advanceTimersByTime(500);

		expect(onLongPress).toHaveBeenCalledWith(100, 100);
		cleanup();
	});

	it('long-press cancels tap callback', () => {
		const onTap = vi.fn();
		const onLongPress = vi.fn();
		const cleanup = createGestureHandler(element, { onTap, onLongPress });

		element.dispatchEvent(createTouchEvent('touchstart', 100, 100));
		vi.advanceTimersByTime(500);
		element.dispatchEvent(createTouchEvent('touchend', 100, 100));

		expect(onLongPress).toHaveBeenCalledOnce();
		expect(onTap).not.toHaveBeenCalled();
		cleanup();
	});

	it('vertical movement cancels long-press', () => {
		const onLongPress = vi.fn();
		const cleanup = createGestureHandler(element, { onLongPress });

		element.dispatchEvent(createTouchEvent('touchstart', 100, 100));
		element.dispatchEvent(createTouchEvent('touchmove', 100, 120));
		vi.advanceTimersByTime(500);

		expect(onLongPress).not.toHaveBeenCalled();
		cleanup();
	});

	it('detects swipe-left — horizontal movement > 50px, vertical < 30px, deltaX < 0', () => {
		const onSwipeLeft = vi.fn();
		const cleanup = createGestureHandler(element, { onSwipeLeft });

		element.dispatchEvent(createTouchEvent('touchstart', 200, 100));
		vi.advanceTimersByTime(200);
		element.dispatchEvent(createTouchEvent('touchend', 100, 110));

		expect(onSwipeLeft).toHaveBeenCalledOnce();
		cleanup();
	});

	it('detects swipe-right — horizontal movement > 50px, vertical < 30px, deltaX > 0', () => {
		const onSwipeRight = vi.fn();
		const cleanup = createGestureHandler(element, { onSwipeRight });

		element.dispatchEvent(createTouchEvent('touchstart', 100, 100));
		vi.advanceTimersByTime(200);
		element.dispatchEvent(createTouchEvent('touchend', 200, 110));

		expect(onSwipeRight).toHaveBeenCalledOnce();
		cleanup();
	});

	it('does not detect swipe when vertical movement exceeds threshold', () => {
		const onSwipeLeft = vi.fn();
		const cleanup = createGestureHandler(element, { onSwipeLeft });

		element.dispatchEvent(createTouchEvent('touchstart', 200, 100));
		vi.advanceTimersByTime(200);
		element.dispatchEvent(createTouchEvent('touchend', 100, 150));

		expect(onSwipeLeft).not.toHaveBeenCalled();
		cleanup();
	});

	it('cleanup function removes all event listeners', () => {
		const onTap = vi.fn();
		const cleanup = createGestureHandler(element, { onTap });

		cleanup();

		element.dispatchEvent(createTouchEvent('touchstart', 100, 100));
		vi.advanceTimersByTime(100);
		element.dispatchEvent(createTouchEvent('touchend', 100, 100));

		expect(onTap).not.toHaveBeenCalled();
	});

	it('touchcancel clears state', () => {
		const onLongPress = vi.fn();
		const cleanup = createGestureHandler(element, { onLongPress });

		element.dispatchEvent(createTouchEvent('touchstart', 100, 100));
		element.dispatchEvent(createTouchEvent('touchcancel', 100, 100));
		vi.advanceTimersByTime(500);

		expect(onLongPress).not.toHaveBeenCalled();
		cleanup();
	});
});
