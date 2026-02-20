import type { Service, HealthStatus, ConnectionStatus } from './types';

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
let sortOrder = $state<string[]>([]);
let initialNeedsAttentionKeys = $state(new Set<string>());

function computeSortOrder(svcMap: Map<string, Service>): string[] {
	return [...svcMap.values()]
		.sort((a, b) => {
			const pri = statusPriority[a.status] - statusPriority[b.status];
			if (pri !== 0) return pri;
			return a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' });
		})
		.map((s) => `${s.namespace}/${s.name}`);
}

function computeNeedsAttentionKeys(svcMap: Map<string, Service>): Set<string> {
	return new Set(
		[...svcMap.values()]
			.filter(
				(s) => s.status === 'unhealthy' || s.status === 'authBlocked' || s.status === 'unknown'
			)
			.map((s) => `${s.namespace}/${s.name}`)
	);
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

	for (const key of sortOrder) {
		const s = services.get(key);
		if (!s) continue;

		if (initialNeedsAttentionKeys.has(key)) {
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

// Mutation functions (called by sseClient only)
export function replaceAll(newServices: Service[], newAppVersion: string): void {
	services = new Map(newServices.map((s) => [`${s.namespace}/${s.name}`, s]));
	appVersion = newAppVersion;
	sortOrder = computeSortOrder(services);
	initialNeedsAttentionKeys = computeNeedsAttentionKeys(services);
	lastUpdated = new Date();
}

export function addOrUpdate(service: Service): void {
	const key = `${service.namespace}/${service.name}`;
	const isNew = !services.has(key);
	const updated = new Map(services);
	updated.set(key, service);
	services = updated;
	if (isNew) {
		sortOrder = computeSortOrder(services);
		if (service.status !== 'healthy') {
			initialNeedsAttentionKeys.add(key);
			initialNeedsAttentionKeys = new Set(initialNeedsAttentionKeys);
		}
	}
	lastUpdated = new Date();
}

export function remove(namespace: string, name: string): void {
	const key = `${namespace}/${name}`;
	const updated = new Map(services);
	updated.delete(key);
	services = updated;
	sortOrder = sortOrder.filter((k) => k !== key);
	initialNeedsAttentionKeys.delete(key);
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
	sortOrder = [];
	initialNeedsAttentionKeys = new Set();
	appVersion = '';
}
