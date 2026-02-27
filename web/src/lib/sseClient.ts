import {
	replaceAll,
	addOrUpdate,
	remove,
	setConnectionStatus,
	setK8sStatus,
	setConfigErrors,
	getK8sConnected
} from './serviceStore.svelte';
import { loadCustomBindings } from './keyboardStore.svelte';
import { saveLastKnownState } from './offlineCache';
import { setLastSyncTime } from './connectivityStore.svelte';
import { resolveApiUrl } from './basePath';
import type { HealthStatus, Service, K8sStatusPayload, ServiceSource, KeyboardConfig } from './types';

let eventSource: EventSource | null = null;
let onlineReconnectHandler: (() => void) | null = null;
type StatePayload = {
	services: Service[];
	appVersion?: string;
	k8sConnected?: boolean;
	k8sLastEvent?: string | null;
	healthCheckIntervalMs?: number;
	configErrors?: string[];
	keyboard?: KeyboardConfig;
};

function closeActiveConnection(): void {
	eventSource?.close();
	eventSource = null;
	if (onlineReconnectHandler && typeof window !== 'undefined') {
		window.removeEventListener('online', onlineReconnectHandler);
		onlineReconnectHandler = null;
	}
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
	return value === 'healthy' || value === 'degraded' || value === 'unhealthy' || value === 'unknown';
}

function isNullableNumber(value: unknown): value is number | null {
	return value === null || typeof value === 'number';
}

function isNullableString(value: unknown): value is string | null {
	return value === null || typeof value === 'string';
}

function isOptionalServiceSource(value: unknown): value is ServiceSource | undefined {
	return value === undefined || value === 'kubernetes' || value === 'config';
}

function isNullableISODateString(value: unknown): value is string | null {
	return (
		value === null ||
		(typeof value === 'string' && !Number.isNaN(new Date(value).getTime()))
	);
}

function isPodDiagnostic(value: unknown): value is { reason: string | null; restartCount: number } {
	if (!isRecord(value)) return false;
	return (
		isNullableString(value.reason) &&
		typeof value.restartCount === 'number' &&
		Number.isInteger(value.restartCount) &&
		value.restartCount >= 0
	);
}

function isNullablePodDiagnostic(value: unknown): boolean {
	return value === null || isPodDiagnostic(value);
}

function isNullableGitOpsStatus(value: unknown): boolean {
	if (value === null || value === undefined) return true;
	if (!isRecord(value)) return false;
	return (
		typeof value.reconciliationState === 'string' &&
		isNullableString(value.lastTransitionTime) &&
		typeof value.message === 'string' &&
		typeof value.sourceType === 'string'
	);
}

function isService(value: unknown): value is Service {
	if (!isRecord(value)) return false;

	return (
		typeof value.name === 'string' &&
		typeof value.displayName === 'string' &&
		typeof value.namespace === 'string' &&
		typeof value.group === 'string' &&
		typeof value.url === 'string' &&
		isOptionalServiceSource(value.source) &&
		(value.icon === undefined || isNullableString(value.icon)) &&
		isHealthStatus(value.status) &&
		isHealthStatus(value.compositeStatus) &&
			isNullableNumber(value.readyEndpoints) &&
			isNullableNumber(value.totalEndpoints) &&
			typeof value.authGuarded === 'boolean' &&
			isNullableNumber(value.httpCode) &&
			isNullableNumber(value.responseTimeMs) &&
			isNullableISODateString(value.lastChecked) &&
			isNullableISODateString(value.lastStateChange) &&
			isNullableString(value.errorSnippet) &&
			isNullablePodDiagnostic(value.podDiagnostic) &&
			isNullableGitOpsStatus(value.gitopsStatus)
	);
}

function isKeyboardConfig(value: unknown): value is KeyboardConfig {
	if (!isRecord(value)) return false;
	if (typeof value.mod !== 'string') return false;
	if (!isRecord(value.bindings)) return false;
	return Object.values(value.bindings).every((v) => typeof v === 'string');
}

function isStatePayload(value: unknown): value is StatePayload {
	if (!isRecord(value) || !Array.isArray(value.services)) return false;
	if (value.appVersion !== undefined && typeof value.appVersion !== 'string') return false;
	if (value.k8sConnected !== undefined && typeof value.k8sConnected !== 'boolean') return false;
	if (value.k8sLastEvent !== undefined && !isNullableISODateString(value.k8sLastEvent))
		return false;
	if (
		value.healthCheckIntervalMs !== undefined &&
		(typeof value.healthCheckIntervalMs !== 'number' ||
			!Number.isInteger(value.healthCheckIntervalMs) ||
			value.healthCheckIntervalMs <= 0)
	)
		return false;
	if (
		value.configErrors !== undefined &&
		(!Array.isArray(value.configErrors) ||
			!value.configErrors.every((e: unknown) => typeof e === 'string'))
	)
		return false;
	if (value.keyboard !== undefined && !isKeyboardConfig(value.keyboard)) return false;
	return value.services.every((service) => isService(service));
}

function isRemovedPayload(value: unknown): value is { namespace: string; name: string } {
	if (!isRecord(value)) return false;
	return typeof value.namespace === 'string' && typeof value.name === 'string';
}

function isK8sStatusPayload(value: unknown): value is K8sStatusPayload {
	if (!isRecord(value)) return false;
	return typeof value.k8sConnected === 'boolean' && isNullableISODateString(value.k8sLastEvent);
}

export function connect(): void {
	closeActiveConnection();

	setConnectionStatus('connecting');
	const sseUrl = resolveApiUrl('api/events');
	const source = new EventSource(sseUrl);
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
		if (payload.k8sConnected !== undefined) {
			setK8sStatus(payload.k8sConnected, payload.k8sLastEvent ?? null);
		} else if (payload.k8sLastEvent !== undefined) {
			setK8sStatus(getK8sConnected(), payload.k8sLastEvent ?? null);
		}
		setConfigErrors(payload.configErrors ?? []);
<<<<<<< HEAD
		if (payload.keyboard) {
			loadCustomBindings(payload.keyboard);
		}

		// Persist state for offline access
		const now = new Date().toISOString();
		setLastSyncTime(now);
		saveLastKnownState(payload.services);
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

	// Reconnect immediately when connectivity returns
	if (typeof window !== 'undefined') {
		onlineReconnectHandler = () => {
			if (eventSource && eventSource.readyState !== EventSource.OPEN) {
				connect();
			}
		};
		window.addEventListener('online', onlineReconnectHandler);
	}
}

export function disconnect(): void {
	closeActiveConnection();
}
