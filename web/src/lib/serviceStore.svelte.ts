import type { Service, ServiceGroup, HealthStatus, ConnectionStatus } from './types';
import { DEFAULT_HEALTH_CHECK_INTERVAL_MS } from './types';

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
let appVersion = $state<string>('');
let k8sConnected = $state<boolean>(false);
let k8sLastEvent = $state<Date | null>(null);
let healthCheckIntervalMs = $state<number>(DEFAULT_HEALTH_CHECK_INTERVAL_MS);
let groupCollapseOverrides = $state(new Map<string, boolean>());

function pruneGroupCollapseOverrides(nextServices: Map<string, Service>): void {
	if (groupCollapseOverrides.size === 0) return;

	// Optimization: Instead of building a Set of all groups in N services,
	// just check if each of the M overrides still has at least one service.
	let removedAny = false;
	const pruned = new Map<string, boolean>();
	const servicesArray = [...nextServices.values()];

	for (const [groupName, override] of groupCollapseOverrides) {
		const stillExists = servicesArray.some((s) => s.group === groupName);
		if (stillExists) {
			pruned.set(groupName, override);
		} else {
			removedAny = true;
		}
	}

	if (removedAny) {
		groupCollapseOverrides = pruned;
	}
}

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

const serviceGroups = $derived.by<ServiceGroup[]>(() => {
	const groupMap = new Map<string, Service[]>();
	for (const svc of services.values()) {
		const g = svc.group;
		if (!groupMap.has(g)) groupMap.set(g, []);
		groupMap.get(g)!.push(svc);
	}

	const groups: ServiceGroup[] = [];
	for (const [name, svcs] of groupMap) {
		const sorted = [...svcs].sort((a, b) => {
			const pri = statusPriority[a.status] - statusPriority[b.status];
			if (pri !== 0) return pri;
			return a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' });
		});

		const groupCounts = {
			healthy: svcs.filter((s) => s.status === 'healthy').length,
			unhealthy: svcs.filter((s) => s.status === 'unhealthy').length,
			authBlocked: svcs.filter((s) => s.status === 'authBlocked').length,
			unknown: svcs.filter((s) => s.status === 'unknown').length
		};

		const hasProblems =
			groupCounts.unhealthy > 0 || groupCounts.authBlocked > 0 || groupCounts.unknown > 0;

		const override = groupCollapseOverrides.get(name);
		const expanded = override !== undefined ? override : hasProblems;

		groups.push({ name, services: sorted, counts: groupCounts, hasProblems, expanded });
	}

	groups.sort((a, b) => {
		// Tier 1: Any unhealthy services? Sort by count descending
		if (a.counts.unhealthy !== b.counts.unhealthy) {
			return b.counts.unhealthy - a.counts.unhealthy;
		}

		// Tier 2: Any other problems (auth-blocked, unknown)?
		if (a.hasProblems !== b.hasProblems) {
			return a.hasProblems ? -1 : 1;
		}

		// Tier 3: Alphabetical by name (case-insensitive)
		return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
	});

	return groups;
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
export function getServiceGroups(): ServiceGroup[] {
	return serviceGroups;
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
	const nextServices = new Map(newServices.map((s) => [`${s.namespace}/${s.name}`, s]));
	services = nextServices;
	pruneGroupCollapseOverrides(nextServices);
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
	pruneGroupCollapseOverrides(updated);
}

export function setConnectionStatus(status: ConnectionStatus): void {
	connectionStatus = status;
}

export function toggleGroupCollapse(groupName: string): void {
	const group = serviceGroups.find((g) => g.name === groupName);
	if (!group) return;
	const newOverrides = new Map(groupCollapseOverrides);
	newOverrides.set(groupName, !group.expanded);
	groupCollapseOverrides = newOverrides;
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
	groupCollapseOverrides = new Map();
}
