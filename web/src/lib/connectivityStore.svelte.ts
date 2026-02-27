let isOnline = $state<boolean>(typeof navigator !== 'undefined' ? navigator.onLine : true);
let lastSyncTime = $state<string | null>(null);

let onlineHandler: (() => void) | null = null;
let offlineHandler: (() => void) | null = null;

export function initConnectivityListener(): void {
	if (typeof window === 'undefined') return;

	isOnline = navigator.onLine;

	onlineHandler = () => {
		isOnline = true;
	};
	offlineHandler = () => {
		isOnline = false;
	};

	window.addEventListener('online', onlineHandler);
	window.addEventListener('offline', offlineHandler);
}

export function destroyConnectivityListener(): void {
	if (onlineHandler) {
		window.removeEventListener('online', onlineHandler);
	}
	if (offlineHandler) {
		window.removeEventListener('offline', offlineHandler);
	}
	onlineHandler = null;
	offlineHandler = null;
}

export function getIsOnline(): boolean {
	return isOnline;
}

export function getLastSyncTime(): string | null {
	return lastSyncTime;
}

export function setLastSyncTime(time: string): void {
	lastSyncTime = time;
}

export function _resetForTesting(): void {
	isOnline = typeof navigator !== 'undefined' ? navigator.onLine : true;
	lastSyncTime = null;
	destroyConnectivityListener();
}
