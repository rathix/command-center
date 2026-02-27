import type {
	TalosNode,
	NodeMetrics,
	NodesResponse,
	MetricsHistoryResponse,
	OperationResult,
	UpgradeInfoResult
} from './types';

// Internal reactive state
let nodes = $state<TalosNode[]>([]);
let lastPoll = $state<string | null>(null);
let error = $state<string | null>(null);
let stale = $state(false);
let configured = $state(false);
let loading = $state(false);
let metricsHistory = $state(new Map<string, NodeMetrics[]>());
let pendingOperation = $state<{ node: string; type: 'reboot' | 'upgrade' } | null>(null);

let pollTimer: ReturnType<typeof setInterval> | null = null;

export async function fetchNodes(): Promise<void> {
	loading = true;
	try {
		const res = await fetch('/api/nodes');
		if (!res.ok) {
			error = `HTTP ${res.status}`;
			stale = true;
			return;
		}
		const data: NodesResponse = await res.json();
		configured = data.configured;
		if (data.nodes) {
			nodes = data.nodes;
		} else {
			nodes = [];
		}
		lastPoll = data.lastPoll || null;
		error = data.error || null;
		stale = data.stale || false;
	} catch (e) {
		error = e instanceof Error ? e.message : 'Unknown error';
		stale = true;
	} finally {
		loading = false;
	}
}

export function startPolling(intervalMs: number): void {
	stopPolling();
	fetchNodes();
	pollTimer = setInterval(fetchNodes, intervalMs);
}

export function stopPolling(): void {
	if (pollTimer !== null) {
		clearInterval(pollTimer);
		pollTimer = null;
	}
}

export async function fetchMetricsHistory(nodeName: string): Promise<void> {
	try {
		const res = await fetch(`/api/nodes/${encodeURIComponent(nodeName)}/metrics`);
		if (!res.ok) return;
		const data: MetricsHistoryResponse = await res.json();
		const next = new Map(metricsHistory);
		next.set(nodeName, data.history);
		metricsHistory = next;
	} catch {
		// Silently fail — metrics history is non-critical
	}
}

export async function rebootNode(
	nodeName: string
): Promise<OperationResult> {
	pendingOperation = { node: nodeName, type: 'reboot' };
	try {
		const res = await fetch(`/api/talos/${encodeURIComponent(nodeName)}/reboot`, {
			method: 'POST'
		});
		const data: OperationResult = await res.json();
		return data;
	} catch (e) {
		return {
			success: false,
			error: e instanceof Error ? e.message : 'Unknown error'
		};
	} finally {
		pendingOperation = null;
	}
}

export async function upgradeNode(
	nodeName: string,
	targetVersion: string
): Promise<OperationResult> {
	pendingOperation = { node: nodeName, type: 'upgrade' };
	try {
		const res = await fetch(`/api/talos/${encodeURIComponent(nodeName)}/upgrade`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ targetVersion })
		});
		const data: OperationResult = await res.json();
		return data;
	} catch (e) {
		return {
			success: false,
			error: e instanceof Error ? e.message : 'Unknown error'
		};
	} finally {
		pendingOperation = null;
	}
}

export async function fetchUpgradeInfo(
	nodeName: string
): Promise<UpgradeInfoResult | null> {
	try {
		const res = await fetch(`/api/talos/${encodeURIComponent(nodeName)}/upgrade-info`);
		if (!res.ok) return null;
		return await res.json();
	} catch {
		return null;
	}
}

// Getter functions (Svelte 5 runes require functions, not direct exports)
export function getNodes(): TalosNode[] {
	return nodes;
}

export function getLastPoll(): string | null {
	return lastPoll;
}

export function getError(): string | null {
	return error;
}

export function isStale(): boolean {
	return stale;
}

export function isConfigured(): boolean {
	return configured;
}

export function isLoading(): boolean {
	return loading;
}

export function getMetricsHistory(nodeName: string): NodeMetrics[] {
	return metricsHistory.get(nodeName) ?? [];
}

export function getPendingOperation(): { node: string; type: 'reboot' | 'upgrade' } | null {
	return pendingOperation;
}

// Test helper — resets all state to initial values
export function _resetForTesting(): void {
	nodes = [];
	lastPoll = null;
	error = null;
	stale = false;
	configured = false;
	loading = false;
	metricsHistory = new Map();
	pendingOperation = null;
	stopPolling();
}
