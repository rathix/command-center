import type { Service, HealthStatus, ConnectionStatus } from './types';

const statusPriority: Record<HealthStatus, number> = {
	unhealthy: 0,
	authBlocked: 1,
	unknown: 2,
	healthy: 3
};

// Internal reactive state
let services = $state(new Map<string, Service>());
let connectionStatus = $state<ConnectionStatus>('connecting');
let lastUpdated = $state<Date | null>(null);

// Internal derived computations
const sortedServices = $derived.by(() => {
	return [...services.values()].sort((a, b) => {
		const pri = statusPriority[a.status] - statusPriority[b.status];
		if (pri !== 0) return pri;
		return a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' });
	});
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
		(s) => s.status === 'unhealthy' || s.status === 'authBlocked'
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

// Mutation functions (called by sseClient only)
export function replaceAll(newServices: Service[]): void {
	services = new Map(newServices.map((s) => [`${s.namespace}/${s.name}`, s]));
	lastUpdated = new Date();
}

export function addOrUpdate(service: Service): void {
	const key = `${service.namespace}/${service.name}`;
	services.set(key, service);
	lastUpdated = new Date();
}

export function remove(namespace: string, name: string): void {
	const key = `${namespace}/${name}`;
	services.delete(key);
	lastUpdated = new Date();
}

export function setConnectionStatus(status: ConnectionStatus): void {
	connectionStatus = status;
}

// Test helper — resets all state to initial values
export function _resetForTesting(): void {
	services = new Map();
	connectionStatus = 'connecting';
	lastUpdated = null;
}
