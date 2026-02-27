<script lang="ts">
	import {
		getNodes,
		isConfigured,
		isStale,
		isLoading,
		getError,
		getMetricsHistory as getNodeMetricsHistory,
		startPolling,
		stopPolling,
		fetchMetricsHistory,
		rebootNode,
		upgradeNode,
		fetchUpgradeInfo,
		getPendingOperation
	} from '$lib/nodeStore.svelte';
	import { formatRelativeTime } from '$lib/formatRelativeTime';
	import Sparkline from './Sparkline.svelte';
	import ConfirmationModal from './ConfirmationModal.svelte';
	import type { TalosNode } from '$lib/types';

	const POLL_INTERVAL_MS = 30_000;

	let modal = $state<{
		node: TalosNode;
		type: 'reboot' | 'upgrade';
		upgradeInfo?: { currentVersion: string; targetVersion: string };
	} | null>(null);

	let operationResults = $state(new Map<string, { success: boolean; message: string; timeout: ReturnType<typeof setTimeout> }>());

	$effect(() => {
		startPolling(POLL_INTERVAL_MS);
		return () => stopPolling();
	});

	// Fetch metrics history for each node on nodes change
	$effect(() => {
		const nodes = getNodes();
		for (const node of nodes) {
			fetchMetricsHistory(node.name);
		}
	});

	function metricsColor(percent: number): string {
		if (percent > 85) return 'text-red';
		if (percent >= 60) return 'text-yellow';
		return 'text-green';
	}

	function healthBadgeClass(health: string): string {
		switch (health) {
			case 'ready':
				return 'bg-green text-crust';
			case 'not-ready':
				return 'bg-yellow text-crust';
			case 'unreachable':
				return 'bg-red text-crust';
			default:
				return 'bg-surface-1 text-text';
		}
	}

	async function handleRebootClick(node: TalosNode) {
		modal = { node, type: 'reboot' };
	}

	async function handleUpgradeClick(node: TalosNode) {
		const info = await fetchUpgradeInfo(node.name);
		if (info) {
			modal = { node, type: 'upgrade', upgradeInfo: info };
		}
	}

	async function handleConfirm() {
		if (!modal) return;
		const { node, type, upgradeInfo } = modal;
		modal = null;

		let result;
		if (type === 'reboot') {
			result = await rebootNode(node.name);
		} else {
			result = await upgradeNode(node.name, upgradeInfo?.targetVersion ?? '');
		}

		// Clear any existing timeout for this node
		const existing = operationResults.get(node.name);
		if (existing) clearTimeout(existing.timeout);

		const dismissMs = result.success ? 5000 : 10000;
		const timeout = setTimeout(() => {
			const next = new Map(operationResults);
			next.delete(node.name);
			operationResults = next;
		}, dismissMs);

		const next = new Map(operationResults);
		next.set(node.name, {
			success: result.success,
			message: result.success ? (result.message ?? 'Operation succeeded') : (result.error ?? 'Operation failed'),
			timeout
		});
		operationResults = next;
	}

	function handleCancel() {
		modal = null;
	}

	function isNodePending(nodeName: string): boolean {
		const op = getPendingOperation();
		return op !== null && op.node === nodeName;
	}
</script>

<div class="p-4" data-testid="node-dashboard">
	{#if !isConfigured()}
		<p class="text-subtext-0">Talos not configured</p>
	{:else if isLoading() && getNodes().length === 0}
		<p class="text-subtext-0">Loading nodes...</p>
	{:else}
		{#if isStale()}
			<div class="mb-3 rounded bg-surface-0 px-3 py-2 text-sm text-yellow" data-testid="staleness-indicator">
				Node data may be stale
			</div>
		{/if}

		{#if getError()}
			<div class="mb-3 text-sm text-red" data-testid="error-message">{getError()}</div>
		{/if}

		<div class="grid gap-3">
			{#each getNodes() as node (node.name)}
				<div class="rounded-lg bg-surface-0 p-4" data-testid="node-card">
					<div class="mb-2 flex items-center justify-between">
						<div class="flex items-center gap-2">
							<span class="font-semibold text-text">{node.name}</span>
							<span class="rounded px-2 py-0.5 text-xs font-medium {healthBadgeClass(node.health)}" data-testid="health-badge">
								{node.health}
							</span>
							<span class="rounded bg-surface-1 px-2 py-0.5 text-xs text-subtext-0" data-testid="role-tag">
								{node.role}
							</span>
						</div>
						<div class="flex gap-2">
							<button
								class="rounded bg-surface-0 px-3 py-1 text-xs text-text hover:bg-surface-1 disabled:opacity-50"
								onclick={() => handleRebootClick(node)}
								disabled={isStale() || isNodePending(node.name)}
								type="button"
								data-testid="reboot-btn"
							>
								Reboot
							</button>
							<button
								class="rounded bg-surface-0 px-3 py-1 text-xs text-text hover:bg-surface-1 disabled:opacity-50"
								onclick={() => handleUpgradeClick(node)}
								disabled={isStale() || isNodePending(node.name)}
								type="button"
								data-testid="upgrade-btn"
							>
								Upgrade
							</button>
						</div>
					</div>

					{#if node.metrics}
						<div class="mt-2 grid grid-cols-3 gap-4 text-sm" data-testid="metrics-section">
							<div class="flex items-center gap-2">
								<span class="text-subtext-1">CPU:</span>
								<span class={metricsColor(node.metrics.cpuPercent)} data-testid="cpu-percent">
									{node.metrics.cpuPercent.toFixed(0)}%
								</span>
								<Sparkline
									data={getNodeMetricsHistory(node.name).map((m) => m.cpuPercent)}
									color="var(--color-green)"
								/>
							</div>
							<div class="flex items-center gap-2">
								<span class="text-subtext-1">Memory:</span>
								<span class={metricsColor(node.metrics.memoryPercent)} data-testid="memory-percent">
									{node.metrics.memoryPercent.toFixed(0)}%
								</span>
								<Sparkline
									data={getNodeMetricsHistory(node.name).map((m) => m.memoryPercent)}
									color="var(--color-blue)"
								/>
							</div>
							<div class="flex items-center gap-2">
								<span class="text-subtext-1">Disk:</span>
								<span class={metricsColor(node.metrics.diskPercent)} data-testid="disk-percent">
									{node.metrics.diskPercent.toFixed(0)}%
								</span>
								<Sparkline
									data={getNodeMetricsHistory(node.name).map((m) => m.diskPercent)}
									color="var(--color-mauve)"
								/>
							</div>
						</div>
						<div class="mt-1 text-xs text-subtext-0">
							Last reading: {formatRelativeTime(node.metrics.timestamp)}
						</div>
					{:else}
						<div class="mt-2 text-sm text-subtext-0" data-testid="metrics-unavailable">Metrics unavailable</div>
					{/if}

					{#if operationResults.has(node.name)}
						{@const result = operationResults.get(node.name)!}
						<div
							class="mt-2 text-sm {result.success ? 'text-green' : 'text-red'}"
							data-testid="operation-result"
						>
							{result.message}
						</div>
					{/if}

					{#if isNodePending(node.name)}
						<div class="mt-2 text-xs text-subtext-0" data-testid="pending-indicator">Operation in progress...</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if modal}
	<ConfirmationModal
		title={modal.type === 'reboot' ? `Reboot ${modal.node.name}?` : `Upgrade ${modal.node.name}?`}
		message={modal.type === 'reboot'
			? `This will reboot the node. It will be temporarily unavailable.`
			: `This will upgrade the node to ${modal.upgradeInfo?.targetVersion ?? 'unknown'}.`}
		details={modal.type === 'reboot'
			? { Node: modal.node.name, Status: modal.node.health, Role: modal.node.role }
			: {
					Node: modal.node.name,
					'Current Version': modal.upgradeInfo?.currentVersion ?? 'unknown',
					'Target Version': modal.upgradeInfo?.targetVersion ?? 'unknown'
				}}
		confirmLabel={modal.type === 'reboot' ? 'Reboot' : 'Upgrade'}
		confirmVariant="danger"
		disabled={isNodePending(modal.node.name)}
		onConfirm={handleConfirm}
		onCancel={handleCancel}
	/>
{/if}
