import type { Service, HealthStatus, ConnectionStatus } from './types';
import { DEFAULT_HEALTH_CHECK_INTERVAL_MS } from './types';

const statusPriority: Record<HealthStatus, number> = {
	unhealthy: 0,
	authBlocked: 1,
	unknown: 2,
	healthy: 3
};

export interface GroupedServices {
	needsAttention: Service[];
	healthy: Service[];
}

// Internal reactive state
let services = $state(new Map<string, Service>());
let connectionStatus = $state<ConnectionStatus>('connecting');
let lastUpdated = $state<Date | null>(null);
let appVersion = $state<string>('');
let k8sConnected = $state<boolean>(false);
let k8sLastEvent = $state<Date | null>(null);
let healthCheckIntervalMs = $state<number>(DEFAULT_HEALTH_CHECK_INTERVAL_MS);

function parseLastChecked(value: string | null): Date | null {
	if (!value) return null;
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) return null;
	return parsed;
}

function newestLastChecked(services: Iterable<Service>): Date | null {
	let latest: Date | null = null;
	for (const svc of services) {
		const checkedAt = parseLastChecked(svc.lastChecked);
		if (!checkedAt) continue;
		if (!latest || checkedAt.getTime() > latest.getTime()) {
			latest = checkedAt;
		}
	}
	return latest;
}

// Internal derived computations
const sortedServices = $derived.by(() => {
	return [...services.values()].sort((a, b) => {
		const pri = statusPriority[a.status] - statusPriority[b.status];
		if (pri !== 0) return pri;
		return a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' });
	});
});

const groupedServices = $derived.by<GroupedServices>(() => {
	const needsAttention: Service[] = [];
	const healthy: Service[] = [];

	for (const s of sortedServices) {
		if (s.status === 'unhealthy' || s.status === 'authBlocked' || s.status === 'unknown') {
			needsAttention.push(s);
		} else {
			healthy.push(s);
		}
	}

	return { needsAttention, healthy };
});

const counts = $derived.by(() => {
	const vals = [...services.values()];
	return {
		total: services.size,
		healthy: vals.filter((s) => s.status === 'healthy').length,
		unhealthy: vals.filter((s) => s.status === 'unhealthy').length,
		authBlocked: vals.filter((s) => s.status === 'authBlocked').length,
		unknown: vals.filter((s) => s.status === 'unknown').length
	};
});

const hasProblems = $derived.by(() => {
	return [...services.values()].some(
		(s) => s.status === 'unhealthy' || s.status === 'authBlocked' || s.status === 'unknown'
	);
});

// Exported getter functions for reading state (Svelte 5 requires functions, not direct $derived exports)
export function getSortedServices(): Service[] {
	return sortedServices;
}
export function getCounts() {
	return counts;
}
export function getHasProblems(): boolean {
	return hasProblems;
}
export function getConnectionStatus(): ConnectionStatus {
	return connectionStatus;
}
export function getLastUpdated(): Date | null {
	return lastUpdated;
}
export function getGroupedServices(): GroupedServices {
	return groupedServices;
}
export function getAppVersion(): string {
	return appVersion;
}
export function getK8sConnected(): boolean {
	return k8sConnected;
}
export function getK8sLastEvent(): Date | null {
	return k8sLastEvent;
}
export function getHealthCheckIntervalMs(): number {
	return healthCheckIntervalMs;
}

// Mutation functions (called by sseClient only)
export function replaceAll(newServices: Service[], newAppVersion: string, newHealthCheckIntervalMs?: number): void {
	services = new Map(newServices.map((s) => [`${s.namespace}/${s.name}`, s]));
	appVersion = newAppVersion;
	healthCheckIntervalMs = newHealthCheckIntervalMs ?? DEFAULT_HEALTH_CHECK_INTERVAL_MS;
	lastUpdated = newestLastChecked(services.values());
}

export function addOrUpdate(service: Service): void {
	const updated = new Map(services);
	updated.set(`${service.namespace}/${service.name}`, service);
	services = updated;
	const checkedAt = parseLastChecked(service.lastChecked);
	if (checkedAt && (!lastUpdated || checkedAt.getTime() > lastUpdated.getTime())) {
		lastUpdated = checkedAt;
	}
}

export function remove(namespace: string, name: string): void {
	const updated = new Map(services);
	updated.delete(`${namespace}/${name}`);
	services = updated;
}

export function setConnectionStatus(status: ConnectionStatus): void {
	connectionStatus = status;
}

export function setK8sStatus(connected: boolean, lastEvent: string | null): void {
	k8sConnected = connected;
	k8sLastEvent = lastEvent ? new Date(lastEvent) : null;
}

// Test helper — resets all state to initial values
export function _resetForTesting(): void {
	services = new Map();
	connectionStatus = 'connecting';
	lastUpdated = null;
	appVersion = '';
	k8sConnected = false;
	k8sLastEvent = null;
	healthCheckIntervalMs = DEFAULT_HEALTH_CHECK_INTERVAL_MS;
}
