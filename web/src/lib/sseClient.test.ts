import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import type { Service, HealthStatus } from './types';

// Mock the serviceStore module before importing sseClient
vi.mock('./serviceStore.svelte', () => ({
	replaceAll: vi.fn(),
	addOrUpdate: vi.fn(),
	remove: vi.fn(),
	setConnectionStatus: vi.fn(),
	setK8sStatus: vi.fn(),
	setConfigErrors: vi.fn(),
	getK8sConnected: vi.fn(() => true)
}));

// Import mocked functions for assertions
import {
	replaceAll,
	addOrUpdate,
	remove,
	setConnectionStatus,
	setK8sStatus,
	setConfigErrors,
	getK8sConnected
} from './serviceStore.svelte';

// MockEventSource to simulate browser EventSource
type EventHandler = (e: MessageEvent) => void;

class MockEventSource {
	static readonly CONNECTING = 0;
	static readonly OPEN = 1;
	static readonly CLOSED = 2;
	static instances: MockEventSource[] = [];

	url: string;
	listeners: Record<string, EventHandler[]> = {};
	onopen: (() => void) | null = null;
	onerror: (() => void) | null = null;
	readyState = 0;
	closed = false;

	constructor(url: string) {
		this.url = url;
		MockEventSource.instances.push(this);
	}

	addEventListener(type: string, handler: EventHandler) {
		this.listeners[type] = this.listeners[type] || [];
		this.listeners[type].push(handler);
	}

	emit(type: string, data: string) {
		this.listeners[type]?.forEach((h) => h(new MessageEvent(type, { data })));
	}

	close() {
		this.closed = true;
	}

	static reset() {
		MockEventSource.instances = [];
	}
}

// Install MockEventSource globally
const originalEventSource = globalThis.EventSource;

beforeEach(() => {
	vi.resetModules();
	vi.clearAllMocks();
	MockEventSource.reset();
	globalThis.EventSource = MockEventSource as unknown as typeof EventSource;
});

afterEach(() => {
	globalThis.EventSource = originalEventSource;
});

function makeService(overrides: Partial<Service> & { name: string }): Service {
	return {
		displayName: overrides.displayName ?? overrides.name,
		namespace: 'default',
		group: 'default',
		url: 'https://test.example.com',
		status: 'unknown',
		httpCode: null,
		responseTimeMs: null,
		lastChecked: null,
		lastStateChange: null,
		errorSnippet: null,
		...overrides
	};
}

describe('sseClient', () => {
	describe('connect', () => {
		it('creates EventSource with correct URL', async () => {
			const { connect } = await import('./sseClient');
			connect();
			expect(MockEventSource.instances).toHaveLength(1);
			expect(MockEventSource.instances[0].url).toBe('/api/events');
		});

		it('sets connection status to connecting on init', async () => {
			const { connect } = await import('./sseClient');
			connect();
			expect(setConnectionStatus).toHaveBeenCalledWith('connecting');
		});

		it('sets connection status to connected on open', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			es.onopen!();
			expect(setConnectionStatus).toHaveBeenCalledWith('connected');
		});

		it('sets connection status to reconnecting when onerror fires with readyState CONNECTING', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			es.readyState = MockEventSource.CONNECTING;
			es.onerror!();
			expect(setConnectionStatus).toHaveBeenCalledWith('reconnecting');
		});

		it('sets connection status to disconnected when onerror fires with readyState CLOSED', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			es.readyState = MockEventSource.CLOSED;
			es.onerror!();
			expect(setConnectionStatus).toHaveBeenCalledWith('disconnected');
		});

		it('transitions from reconnecting to connected when onopen fires after reconnect', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			// Simulate connection drop with auto-reconnect in progress
			es.readyState = MockEventSource.CONNECTING;
			es.onerror!();
			expect(setConnectionStatus).toHaveBeenCalledWith('reconnecting');
			// Simulate successful reconnect
			es.readyState = MockEventSource.OPEN;
			es.onopen!();
			expect(setConnectionStatus).toHaveBeenCalledWith('connected');
		});

		it('preserves service data during reconnecting state (services map not cleared)', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			// Simulate having received some service data
			const services = [makeService({ name: 'svc-a' })];
			es.emit('state', JSON.stringify({ services, appVersion: 'v1.0.0' }));
			expect(replaceAll).toHaveBeenCalledWith(services, 'v1.0.0', undefined);
			// Simulate connection drop — should NOT call replaceAll again
			vi.clearAllMocks();
			es.readyState = MockEventSource.CONNECTING;
			es.onerror!();
			expect(setConnectionStatus).toHaveBeenCalledWith('reconnecting');
			expect(replaceAll).not.toHaveBeenCalled();
		});

		it('calls replaceAll when state event is received after reconnect', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			// Simulate reconnect cycle
			es.readyState = MockEventSource.CONNECTING;
			es.onerror!();
			es.readyState = MockEventSource.OPEN;
			es.onopen!();
			vi.clearAllMocks();
			// Simulate state event after reconnect
			const services = [makeService({ name: 'svc-refreshed' })];
			es.emit('state', JSON.stringify({ services, appVersion: 'v2.0.0' }));
			expect(replaceAll).toHaveBeenCalledWith(services, 'v2.0.0', undefined);
		});

		it('closes the previous EventSource when called more than once', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const first = MockEventSource.instances[0];

			connect();
			const second = MockEventSource.instances[1];

			expect(MockEventSource.instances).toHaveLength(2);
			expect(first.closed).toBe(true);
			expect(second.closed).toBe(false);
		});
	});

	describe('state event', () => {
		it('calls replaceAll with parsed services array and appVersion', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			const services = [makeService({ name: 'svc-a' }), makeService({ name: 'svc-b' })];
			const appVersion = 'v1.2.3';
			es.emit('state', JSON.stringify({ services, appVersion }));
			expect(replaceAll).toHaveBeenCalledWith(services, appVersion, undefined);
		});

		it('ignores malformed JSON', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];

			es.emit('state', '{bad json');

			expect(replaceAll).not.toHaveBeenCalled();
		});

		it('ignores invalid payload shape', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];

			es.emit('state', JSON.stringify({ services: [{ name: 'partial' }] }));

			expect(replaceAll).not.toHaveBeenCalled();
		});

			it('ignores state payload when optional k8sConnected has invalid type', async () => {
				const { connect } = await import('./sseClient');
				connect();
				const es = MockEventSource.instances[0];
			const services = [makeService({ name: 'svc-a' })];

			es.emit(
				'state',
				JSON.stringify({ services, appVersion: 'v1.0.0', k8sConnected: 'true' })
			);

				expect(replaceAll).not.toHaveBeenCalled();
				expect(setK8sStatus).not.toHaveBeenCalled();
			});

			it('ignores state payload when healthCheckIntervalMs has invalid type', async () => {
				const { connect } = await import('./sseClient');
				connect();
				const es = MockEventSource.instances[0];
				const services = [makeService({ name: 'svc-a' })];

				es.emit(
					'state',
					JSON.stringify({ services, appVersion: 'v1.0.0', healthCheckIntervalMs: '30000' })
				);

				expect(replaceAll).not.toHaveBeenCalled();
			});

			it('ignores state payload when healthCheckIntervalMs is zero or negative', async () => {
				const { connect } = await import('./sseClient');
				connect();
				const es = MockEventSource.instances[0];
				const services = [makeService({ name: 'svc-a' })];

				es.emit(
					'state',
					JSON.stringify({ services, appVersion: 'v1.0.0', healthCheckIntervalMs: 0 })
				);
				expect(replaceAll).not.toHaveBeenCalled();

				es.emit(
					'state',
					JSON.stringify({ services, appVersion: 'v1.0.0', healthCheckIntervalMs: -100 })
				);
				expect(replaceAll).not.toHaveBeenCalled();
			});

			it('ignores state payload when services is missing or not an array', async () => {
				const { connect } = await import('./sseClient');
				connect();
				const es = MockEventSource.instances[0];

				es.emit('state', JSON.stringify({ appVersion: 'v1.0.0' }));
				expect(replaceAll).not.toHaveBeenCalled();

				es.emit('state', JSON.stringify({ services: 'not-an-array', appVersion: 'v1.0.0' }));
				expect(replaceAll).not.toHaveBeenCalled();
			});

			it('ignores state payload when services array contains invalid objects', async () => {
				const { connect } = await import('./sseClient');
				connect();
				const es = MockEventSource.instances[0];

				es.emit('state', JSON.stringify({ services: [{ name: 'incomplete' }] }));
				expect(replaceAll).not.toHaveBeenCalled();
			});
		});

	describe('discovered event', () => {
		it('calls addOrUpdate with parsed service', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			const service = makeService({ name: 'new-svc' });
			es.emit('discovered', JSON.stringify(service));
			expect(addOrUpdate).toHaveBeenCalledWith(service);
		});

		it('ignores malformed service payloads', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];

			es.emit('discovered', JSON.stringify({ name: 'partial' }));
			expect(addOrUpdate).not.toHaveBeenCalled();

			es.emit('discovered', JSON.stringify(makeService({ name: 'bad-code', httpCode: '200' as unknown as number })));
			expect(addOrUpdate).not.toHaveBeenCalled();

			es.emit('discovered', JSON.stringify(makeService({ name: 'bad-status', status: 'invalid' as unknown as HealthStatus })));
			expect(addOrUpdate).not.toHaveBeenCalled();

			es.emit('discovered', JSON.stringify(makeService({ name: 'bad-time', responseTimeMs: {} as unknown as number })));
			expect(addOrUpdate).not.toHaveBeenCalled();
		});

		it('rejects service payloads missing group field', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];

			const { group, ...serviceWithoutGroup } = makeService({ name: 'no-group' });
			void group;
			es.emit('discovered', JSON.stringify(serviceWithoutGroup));
			expect(addOrUpdate).not.toHaveBeenCalled();
		});
	});

	describe('update event', () => {
		it('calls addOrUpdate with parsed service', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			const service = makeService({ name: 'updated-svc', status: 'healthy' });
			es.emit('update', JSON.stringify(service));
			expect(addOrUpdate).toHaveBeenCalledWith(service);
		});

		it('ignores malformed JSON', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];

			es.emit('update', '{bad json');

			expect(addOrUpdate).not.toHaveBeenCalled();
		});
	});

	describe('removed event', () => {
		it('calls remove with parsed namespace and name', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			es.emit('removed', JSON.stringify({ name: 'old-svc', namespace: 'default' }));
			expect(remove).toHaveBeenCalledWith('default', 'old-svc');
		});

		it('ignores invalid removed payloads', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];

			es.emit('removed', JSON.stringify({ name: 'old-svc' }));

			expect(remove).not.toHaveBeenCalled();
		});
	});

	describe('k8sStatus event', () => {
		it('calls setK8sStatus with parsed payload', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			es.emit('k8sStatus', JSON.stringify({ k8sConnected: true, k8sLastEvent: '2026-02-20T14:30:00Z' }));
			expect(setK8sStatus).toHaveBeenCalledWith(true, '2026-02-20T14:30:00Z');
		});

		it('calls setK8sStatus with false and null', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			es.emit('k8sStatus', JSON.stringify({ k8sConnected: false, k8sLastEvent: null }));
			expect(setK8sStatus).toHaveBeenCalledWith(false, null);
		});

		it('ignores malformed k8sStatus payloads', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			es.emit('k8sStatus', JSON.stringify({ k8sConnected: 'yes' }));
			expect(setK8sStatus).not.toHaveBeenCalled();
		});
	});

		describe('state event with K8s metadata', () => {
		it('extracts k8sConnected from enriched state event', async () => {
			const { connect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			const services = [makeService({ name: 'svc-a' })];
			es.emit(
				'state',
				JSON.stringify({
					services,
					appVersion: 'v1.0.0',
					k8sConnected: true,
					k8sLastEvent: '2026-02-20T14:30:00Z'
				})
			);
			expect(replaceAll).toHaveBeenCalledWith(services, 'v1.0.0', undefined);
			expect(setK8sStatus).toHaveBeenCalledWith(true, '2026-02-20T14:30:00Z');
		});

			it('does not call setK8sStatus when k8sConnected is absent from state event', async () => {
				const { connect } = await import('./sseClient');
				connect();
				const es = MockEventSource.instances[0];
				const services = [makeService({ name: 'svc-a' })];
				es.emit('state', JSON.stringify({ services, appVersion: 'v1.0.0' }));
				expect(replaceAll).toHaveBeenCalled();
				expect(setK8sStatus).not.toHaveBeenCalled();
			});

			it('preserves current k8sConnected when only k8sLastEvent is present', async () => {
				vi.mocked(getK8sConnected).mockReturnValueOnce(false);
				const { connect } = await import('./sseClient');
				connect();
				const es = MockEventSource.instances[0];
				const services = [makeService({ name: 'svc-a' })];

				es.emit(
					'state',
					JSON.stringify({
						services,
						appVersion: 'v1.0.0',
						k8sLastEvent: '2026-02-20T14:30:00Z'
					})
				);

				expect(setK8sStatus).toHaveBeenCalledWith(false, '2026-02-20T14:30:00Z');
			});
		});

	describe('disconnect', () => {
		it('closes the EventSource', async () => {
			const { connect, disconnect } = await import('./sseClient');
			connect();
			const es = MockEventSource.instances[0];
			disconnect();
			expect(es.closed).toBe(true);
		});

		it('is safe to call when not connected', async () => {
			const { disconnect } = await import('./sseClient');
			expect(() => disconnect()).not.toThrow();
		});
	});
});
