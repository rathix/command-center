export type Breakpoint = 'mobile' | 'tablet' | 'desktop';

let currentBreakpoint = $state<Breakpoint>('desktop');

let mobileQuery: MediaQueryList | null = null;
let tabletQuery: MediaQueryList | null = null;
let mobileHandler: ((e: MediaQueryListEvent) => void) | null = null;
let tabletHandler: ((e: MediaQueryListEvent) => void) | null = null;

function computeBreakpoint(): Breakpoint {
	if (mobileQuery && mobileQuery.matches) return 'mobile';
	if (tabletQuery && tabletQuery.matches) return 'tablet';
	return 'desktop';
}

export function initBreakpointListener(): void {
	if (typeof window === 'undefined') return;

	mobileQuery = window.matchMedia('(max-width: 639px)');
	tabletQuery = window.matchMedia('(min-width: 640px) and (max-width: 1023px)');

	currentBreakpoint = computeBreakpoint();

	mobileHandler = () => {
		currentBreakpoint = computeBreakpoint();
	};
	tabletHandler = () => {
		currentBreakpoint = computeBreakpoint();
	};

	mobileQuery.addEventListener('change', mobileHandler);
	tabletQuery.addEventListener('change', tabletHandler);
}

export function destroyBreakpointListener(): void {
	if (mobileQuery && mobileHandler) {
		mobileQuery.removeEventListener('change', mobileHandler);
	}
	if (tabletQuery && tabletHandler) {
		tabletQuery.removeEventListener('change', tabletHandler);
	}
	mobileQuery = null;
	tabletQuery = null;
	mobileHandler = null;
	tabletHandler = null;
}

export function getBreakpoint(): Breakpoint {
	return currentBreakpoint;
}

export function isMobile(): boolean {
	return currentBreakpoint === 'mobile';
}

export function isTablet(): boolean {
	return currentBreakpoint === 'tablet';
}

export function isDesktop(): boolean {
	return currentBreakpoint === 'desktop';
}

export function _resetForTesting(): void {
	currentBreakpoint = 'desktop';
	destroyBreakpointListener();
}
