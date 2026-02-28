import { describe, it, expect, beforeEach, vi } from 'vitest';
import { saveLastKnownState, loadLastKnownState, getLastSyncTime } from './offlineCache';
import type { Service } from './types';

function makeService(overrides: Partial<Service> = {}): Service {
	return {
		name: 'svc-1',
		displayName: 'Service 1',
		namespace: 'default',
		group: 'default',
		url: 'https://svc-1.example.com',
		status: 'healthy',
		compositeStatus: 'healthy',
		readyEndpoints: null,
		totalEndpoints: null,
		authGuarded: false,
		httpCode: 200,
		responseTimeMs: 50,
		lastChecked: '2026-02-20T10:00:00Z',
		lastStateChange: '2026-02-20T09:00:00Z',
		errorSnippet: null,
		podDiagnostic: null,
		gitopsStatus: null,
		...overrides
	} as Service;
}

// Mock Cache API
function createMockCacheStorage() {
	const store = new Map<string, Response>();

	const mockCache = {
		put: vi.fn(async (key: string, response: Response) => {
			// Clone the response so the body can be read again
			store.set(key, response.clone());
		}),
		match: vi.fn(async (key: string) => {
			const resp = store.get(key);
			return resp ? resp.clone() : undefined;
		}),
		delete: vi.fn(async (key: string) => {
			return store.delete(key);
		})
	};

	const mockCaches = {
		open: vi.fn(async () => mockCache)
	};

	return { mockCaches, mockCache, store };
}

describe('offlineCache', () => {
	let mockCacheSetup: ReturnType<typeof createMockCacheStorage>;

	beforeEach(() => {
		mockCacheSetup = createMockCacheStorage();
		vi.stubGlobal('caches', mockCacheSetup.mockCaches);
	});

	it('saveLastKnownState and loadLastKnownState round-trip', async () => {
		const services = [makeService(), makeService({ name: 'svc-2', displayName: 'Service 2' })];
		await saveLastKnownState(services);
		const loaded = await loadLastKnownState();
		expect(loaded).toEqual(services);
	});

	it('loadLastKnownState returns null when cache is empty', async () => {
		const result = await loadLastKnownState();
		expect(result).toBeNull();
	});

	it('getLastSyncTime returns timestamp after save', async () => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-02-27T10:00:00Z'));

		const services = [makeService()];
		await saveLastKnownState(services);
		const syncTime = await getLastSyncTime();
		expect(syncTime).toBe('2026-02-27T10:00:00.000Z');

		vi.useRealTimers();
	});

	it('getLastSyncTime returns null when no state saved', async () => {
		const syncTime = await getLastSyncTime();
		expect(syncTime).toBeNull();
	});

	it('saveLastKnownState does not throw when Cache API unavailable', async () => {
		vi.stubGlobal('caches', {
			open: vi.fn().mockRejectedValue(new Error('Cache not available'))
		});
		await expect(saveLastKnownState([makeService()])).resolves.not.toThrow();
	});

	it('loadLastKnownState returns null when Cache API throws', async () => {
		vi.stubGlobal('caches', {
			open: vi.fn().mockRejectedValue(new Error('Cache not available'))
		});
		const result = await loadLastKnownState();
		expect(result).toBeNull();
	});
});
