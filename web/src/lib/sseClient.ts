import { replaceAll, addOrUpdate, remove, setConnectionStatus } from './serviceStore.svelte';
import type { HealthStatus, Service } from './types';

let eventSource: EventSource | null = null;

function closeActiveConnection(): void {
	eventSource?.close();
	eventSource = null;
}

function parseJson(data: string): unknown | null {
	try {
		return JSON.parse(data);
	} catch {
		return null;
	}
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null;
}

function isHealthStatus(value: unknown): value is HealthStatus {
	return value === 'healthy' || value === 'unhealthy' || value === 'authBlocked' || value === 'unknown';
}

function isNullableNumber(value: unknown): value is number | null {
	return value === null || typeof value === 'number';
}

function isNullableString(value: unknown): value is string | null {
	return value === null || typeof value === 'string';
}

function isService(value: unknown): value is Service {
	if (!isRecord(value)) return false;

	return (
		typeof value.name === 'string' &&
		typeof value.namespace === 'string' &&
		typeof value.url === 'string' &&
		isHealthStatus(value.status) &&
		isNullableNumber(value.httpCode) &&
		isNullableNumber(value.responseTimeMs) &&
		isNullableString(value.lastChecked) &&
		isNullableString(value.lastStateChange) &&
		isNullableString(value.errorSnippet)
	);
}

function isStatePayload(value: unknown): value is { services: Service[] } {
	if (!isRecord(value) || !Array.isArray(value.services)) return false;
	return value.services.every((service) => isService(service));
}

function isRemovedPayload(value: unknown): value is { namespace: string; name: string } {
	if (!isRecord(value)) return false;
	return typeof value.namespace === 'string' && typeof value.name === 'string';
}

export function connect(): void {
	closeActiveConnection();

	setConnectionStatus('connecting');
	const source = new EventSource('/api/events');
	eventSource = source;

	source.onopen = () => {
		if (eventSource === source) {
			setConnectionStatus('connected');
		}
	};

	source.onerror = () => {
		if (eventSource === source) {
			setConnectionStatus('disconnected');
		}
	};

	source.addEventListener('state', (e: MessageEvent) => {
		const payload = parseJson(e.data);
		if (!isStatePayload(payload)) return;
		replaceAll(payload.services);
	});

	source.addEventListener('discovered', (e: MessageEvent) => {
		const service = parseJson(e.data);
		if (!isService(service)) return;
		addOrUpdate(service);
	});

	source.addEventListener('update', (e: MessageEvent) => {
		const service = parseJson(e.data);
		if (!isService(service)) return;
		addOrUpdate(service);
	});

	source.addEventListener('removed', (e: MessageEvent) => {
		const payload = parseJson(e.data);
		if (!isRemovedPayload(payload)) return;
		remove(payload.namespace, payload.name);
	});
}

export function disconnect(): void {
	closeActiveConnection();
}
