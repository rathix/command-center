import type { Service } from './types';

const CACHE_NAME = 'command-center-offline';
const STATE_KEY = '/__offline/last-state';
const SYNC_TIME_KEY = '/__offline/last-sync-time';

export async function saveLastKnownState(services: Service[]): Promise<void> {
	try {
		const cache = await caches.open(CACHE_NAME);
		const stateResponse = new Response(JSON.stringify(services), {
			headers: { 'Content-Type': 'application/json' }
		});
		await cache.put(STATE_KEY, stateResponse);

		const timeResponse = new Response(new Date().toISOString(), {
			headers: { 'Content-Type': 'text/plain' }
		});
		await cache.put(SYNC_TIME_KEY, timeResponse);
	} catch {
		// Cache API unavailable (e.g., non-secure context)
	}
}

export async function loadLastKnownState(): Promise<Service[] | null> {
	try {
		const cache = await caches.open(CACHE_NAME);
		const response = await cache.match(STATE_KEY);
		if (!response) return null;
		const data = await response.json();
		if (!Array.isArray(data)) return null;
		return data as Service[];
	} catch {
		return null;
	}
}

export async function getLastSyncTime(): Promise<string | null> {
	try {
		const cache = await caches.open(CACHE_NAME);
		const response = await cache.match(SYNC_TIME_KEY);
		if (!response) return null;
		return await response.text();
	} catch {
		return null;
	}
}
