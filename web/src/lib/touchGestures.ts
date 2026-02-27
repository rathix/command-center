export type GestureType = 'tap' | 'longpress' | 'swipe-left' | 'swipe-right';

export interface GestureCallbacks {
	onTap?: () => void;
	onLongPress?: (x: number, y: number) => void;
	onSwipeLeft?: () => void;
	onSwipeRight?: () => void;
}

const TAP_MAX_DURATION = 300;
const LONGPRESS_DURATION = 500;
const SWIPE_MIN_DISTANCE = 50;
const SWIPE_MAX_VERTICAL = 30;
const SWIPE_MAX_DURATION = 500;
const MOVEMENT_THRESHOLD = 10;

export function createGestureHandler(
	element: HTMLElement,
	callbacks: GestureCallbacks
): () => void {
	let startX = 0;
	let startY = 0;
	let startTime = 0;
	let longPressTimer: ReturnType<typeof setTimeout> | null = null;
	let gestureRecognized = false;

	function clearLongPressTimer() {
		if (longPressTimer) {
			clearTimeout(longPressTimer);
			longPressTimer = null;
		}
	}

	function handleTouchStart(e: TouchEvent) {
		const touch = e.touches[0];
		if (!touch) return;

		startX = touch.clientX;
		startY = touch.clientY;
		startTime = Date.now();
		gestureRecognized = false;

		if (callbacks.onLongPress) {
			longPressTimer = setTimeout(() => {
				gestureRecognized = true;
				if (navigator.vibrate) {
					navigator.vibrate(50);
				}
				callbacks.onLongPress?.(startX, startY);
			}, LONGPRESS_DURATION);
		}
	}

	function handleTouchMove(e: TouchEvent) {
		const touch = e.touches[0];
		if (!touch) return;

		const dx = Math.abs(touch.clientX - startX);
		const dy = Math.abs(touch.clientY - startY);

		// Cancel long-press if finger moves too much
		if (dx > MOVEMENT_THRESHOLD || dy > MOVEMENT_THRESHOLD) {
			clearLongPressTimer();
		}

		// If vertical movement dominates, let native scroll happen
		if (dy > dx) {
			clearLongPressTimer();
		}
	}

	function handleTouchEnd(e: TouchEvent) {
		clearLongPressTimer();

		if (gestureRecognized) {
			e.preventDefault();
			return;
		}

		const touch = e.changedTouches[0];
		if (!touch) return;

		const dx = touch.clientX - startX;
		const dy = touch.clientY - startY;
		const absDx = Math.abs(dx);
		const absDy = Math.abs(dy);
		const duration = Date.now() - startTime;

		// Check swipe
		if (
			absDx > SWIPE_MIN_DISTANCE &&
			absDy < SWIPE_MAX_VERTICAL &&
			duration < SWIPE_MAX_DURATION
		) {
			e.preventDefault();
			if (dx < 0 && callbacks.onSwipeLeft) {
				callbacks.onSwipeLeft();
			} else if (dx > 0 && callbacks.onSwipeRight) {
				callbacks.onSwipeRight();
			}
			return;
		}

		// Check tap
		if (
			absDx < MOVEMENT_THRESHOLD &&
			absDy < MOVEMENT_THRESHOLD &&
			duration < TAP_MAX_DURATION
		) {
			e.preventDefault();
			callbacks.onTap?.();
		}
	}

	function handleTouchCancel() {
		clearLongPressTimer();
		gestureRecognized = false;
	}

	element.addEventListener('touchstart', handleTouchStart, { passive: true });
	element.addEventListener('touchmove', handleTouchMove, { passive: true });
	element.addEventListener('touchend', handleTouchEnd);
	element.addEventListener('touchcancel', handleTouchCancel);

	return () => {
		clearLongPressTimer();
		element.removeEventListener('touchstart', handleTouchStart);
		element.removeEventListener('touchmove', handleTouchMove);
		element.removeEventListener('touchend', handleTouchEnd);
		element.removeEventListener('touchcancel', handleTouchCancel);
	};
}
