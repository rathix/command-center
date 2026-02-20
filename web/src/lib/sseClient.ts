import { replaceAll, addOrUpdate, remove, setConnectionStatus, setK8sStatus } from './serviceStore.svelte';
import type { HealthStatus, Service, K8sStatusPayload } from './types';

let eventSource: EventSource | null = null;
type StatePayload = {
	services: Service[];
	appVersion?: string;
	k8sConnected?: boolean;
	k8sLastEvent?: string | null;
	healthCheckIntervalMs?: number;
};

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
		typeof value.displayName === 'string' &&
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

function isStatePayload(value: unknown): value is StatePayload {
	if (!isRecord(value) || !Array.isArray(value.services)) return false;
	if (value.appVersion !== undefined && typeof value.appVersion !== 'string') return false;
	if (value.k8sConnected !== undefined && typeof value.k8sConnected !== 'boolean') return false;
	if (
		value.k8sLastEvent !== undefined &&
		value.k8sLastEvent !== null &&
		typeof value.k8sLastEvent !== 'string'
	)
		return false;
	return value.services.every((service) => isService(service));
}

function isRemovedPayload(value: unknown): value is { namespace: string; name: string } {
	if (!isRecord(value)) return false;
	return typeof value.namespace === 'string' && typeof value.name === 'string';
}

function isK8sStatusPayload(value: unknown): value is K8sStatusPayload {
	if (!isRecord(value)) return false;
	return (
		typeof value.k8sConnected === 'boolean' &&
		(value.k8sLastEvent === null || typeof value.k8sLastEvent === 'string')
	);
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
			if (source.readyState === EventSource.CONNECTING) {
				setConnectionStatus('reconnecting');
			} else {
				setConnectionStatus('disconnected');
			}
		}
	};

	source.addEventListener('state', (e: MessageEvent) => {
		const payload = parseJson(e.data);
		if (!isStatePayload(payload)) return;
		replaceAll(payload.services, payload.appVersion ?? '', payload.healthCheckIntervalMs);
		if (payload.k8sConnected !== undefined || payload.k8sLastEvent !== undefined) {
			setK8sStatus(payload.k8sConnected ?? false, payload.k8sLastEvent ?? null);
		}
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

	source.addEventListener('k8sStatus', (e: MessageEvent) => {
		const payload = parseJson(e.data);
		if (!isK8sStatusPayload(payload)) return;
		setK8sStatus(payload.k8sConnected, payload.k8sLastEvent);
	});
}

export function disconnect(): void {
	closeActiveConnection();
}
