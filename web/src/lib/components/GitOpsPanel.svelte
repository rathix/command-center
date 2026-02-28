<script lang="ts">
	import { onDestroy } from 'svelte';
	import type {
		Service,
		GitOpsStatus,
		ReconciliationState
	} from '$lib/types';
	import { getServiceGroups } from '$lib/serviceStore.svelte';

	interface GitOpsStatusApiResponse {
		configured: boolean;
		provider?: string;
		repository?: string;
	}

	interface CommitData {
		sha: string;
		message: string;
		author: string;
		timestamp: string;
	}

	interface Correlation {
		serviceName: string;
		healthChangeTime: string;
		deploymentTime: string;
		commitSha: string;
	}

	const LOOKBACK_WINDOW_MS = 10 * 60 * 1000; // 10 minutes

	let configured = $state<boolean | null>(null);
	let repository = $state('');
	let commits = $state<CommitData[]>([]);
	let error = $state('');
	let loading = $state(true);

	// Rollback state (wired in Story 14.3)
	let rollbackSha = $state<string | null>(null);
	let rollbackCommit = $state<CommitData | null>(null);
	let rollbackLoading = $state(false);
	let rollbackError = $state('');
	let rollbackSuccess = $state('');

	const stateColorMap: Record<ReconciliationState, string> = {
		synced: 'text-health-ok',
		progressing: 'text-health-warning',
		failed: 'text-health-error',
		suspended: 'text-subtext-0'
	};

	const stateBgMap: Record<ReconciliationState, string> = {
		synced: 'bg-health-ok/10',
		progressing: 'bg-health-warning/10',
		failed: 'bg-health-error/10',
		suspended: 'bg-surface-0'
	};

	// Gather all services with gitops status
	const gitopsServices = $derived.by(() => {
		const result: { service: Service; gitops: GitOpsStatus }[] = [];
		for (const group of getServiceGroups()) {
			for (const svc of group.services) {
				if (svc.gitopsStatus) {
					result.push({ service: svc, gitops: svc.gitopsStatus });
				}
			}
		}
		return result;
	});

	// Compute correlations between health changes and deployments
	const correlations = $derived.by(() => {
		const result: Correlation[] = [];
		const now = Date.now();

		for (const { service, gitops } of gitopsServices) {
			if (!service.lastStateChange || !gitops.lastTransitionTime) continue;

			const healthChangeMs = new Date(service.lastStateChange).getTime();
			const deploymentMs = new Date(gitops.lastTransitionTime).getTime();

			// Both must be within lookback window of each other
			if (Math.abs(healthChangeMs - deploymentMs) <= LOOKBACK_WINDOW_MS) {
				// And at least one must be recent (within lookback window from now)
				if (now - healthChangeMs <= LOOKBACK_WINDOW_MS || now - deploymentMs <= LOOKBACK_WINDOW_MS) {
					result.push({
						serviceName: service.displayName || service.name,
						healthChangeTime: service.lastStateChange,
						deploymentTime: gitops.lastTransitionTime,
						commitSha: ''
					});
				}
			}
		}
		return result;
	});

	function isCorrelatedCommit(commitTimestamp: string): boolean {
		const commitMs = new Date(commitTimestamp).getTime();
		for (const corr of correlations) {
			const deployMs = new Date(corr.deploymentTime).getTime();
			if (Math.abs(commitMs - deployMs) <= LOOKBACK_WINDOW_MS) {
				return true;
			}
		}
		return false;
	}

	async function fetchStatus() {
		try {
			const resp = await fetch('/api/gitops/status');
			const json = await resp.json();
			if (json.ok && json.data) {
				const data = json.data as GitOpsStatusApiResponse;
				configured = data.configured;
				repository = data.repository ?? '';
			}
		} catch {
			configured = false;
		} finally {
			loading = false;
		}
	}

	async function fetchCommits() {
		if (!configured) return;
		try {
			const resp = await fetch('/api/gitops/commits');
			if (resp.status === 429) {
				error = 'GitHub API rate limited. Try again later.';
				return;
			}
			const json = await resp.json();
			if (json.ok && json.data) {
				commits = json.data.commits ?? [];
				error = '';
			} else {
				error = json.error || 'Failed to fetch commits';
			}
		} catch {
			error = 'Failed to connect to API';
		}
	}

	function openRollback(commit: CommitData) {
		rollbackSha = commit.sha;
		rollbackCommit = commit;
		rollbackError = '';
		rollbackSuccess = '';
	}

	function closeRollback() {
		rollbackSha = null;
		rollbackCommit = null;
		rollbackLoading = false;
		rollbackError = '';
	}

	async function confirmRollback() {
		if (!rollbackSha) return;
		rollbackLoading = true;
		rollbackError = '';

		try {
			const resp = await fetch('/api/gitops/rollback', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ sha: rollbackSha })
			});
			const json = await resp.json();
			if (json.ok && json.data) {
				rollbackSuccess = `Revert commit created: ${json.data.revertSha?.slice(0, 7)}`;
				closeRollback();
				fetchCommits();
			} else if (resp.status === 429) {
				rollbackError = 'GitHub API rate limited. Try again in a few seconds.';
			} else {
				rollbackError = json.error || 'Rollback failed';
			}
		} catch {
			rollbackError = 'Failed to connect to API';
		} finally {
			rollbackLoading = false;
		}
	}

	function formatTime(iso: string): string {
		try {
			return new Date(iso).toLocaleString();
		} catch {
			return iso;
		}
	}

	// Fetch on mount
	fetchStatus().then(() => {
		if (configured) fetchCommits();
	});

	// Poll commits every 60s
	const pollInterval = setInterval(() => {
		if (configured) fetchCommits();
	}, 60_000);

	onDestroy(() => clearInterval(pollInterval));
</script>

<div class="flex flex-col gap-4 p-4" data-testid="gitops-panel">
	{#if loading}
		<p class="text-sm text-subtext-0">Loading GitOps status...</p>
	{:else if configured === false}
		<div class="text-sm text-subtext-0">
			<p class="font-semibold">GitOps is not configured</p>
			<p class="mt-1">
				Add a <code class="bg-surface-0 px-1 rounded">gitops</code> section to your YAML config file to enable GitOps integration.
			</p>
		</div>
	{:else}
		{#if rollbackSuccess}
			<div class="rounded border border-health-ok/30 bg-health-ok/10 p-2 text-sm text-health-ok">
				{rollbackSuccess}
			</div>
		{/if}

		<!-- Reconciliation State Table -->
		<div>
			<h3 class="text-sm font-semibold text-text mb-2">Reconciliation State</h3>
			{#if gitopsServices.length === 0}
				<p class="text-sm text-subtext-0">No services with GitOps data</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="w-full text-sm" data-testid="reconciliation-table">
						<thead>
							<tr class="text-left text-subtext-0 border-b border-surface-0">
								<th class="py-1 pr-3">Service</th>
								<th class="py-1 pr-3">State</th>
								<th class="py-1 pr-3">Last Transition</th>
								<th class="py-1">Message</th>
							</tr>
						</thead>
						<tbody>
							{#each gitopsServices as { service, gitops } (service.name)}
								<tr class="border-b border-surface-0/50">
									<td class="py-1 pr-3 text-text">{service.displayName || service.name}</td>
									<td class="py-1 pr-3">
										<span class="inline-block rounded px-1.5 py-0.5 text-xs font-medium {stateColorMap[gitops.reconciliationState]} {stateBgMap[gitops.reconciliationState]}">
											{gitops.reconciliationState}
										</span>
									</td>
									<td class="py-1 pr-3 text-subtext-0">
										{gitops.lastTransitionTime ? formatTime(gitops.lastTransitionTime) : '—'}
									</td>
									<td class="py-1 text-subtext-0 truncate max-w-[300px]" title={gitops.message}>
										{gitops.message || '—'}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>

		<!-- Correlations -->
		{#if correlations.length > 0}
			<div class="rounded border border-health-warning/30 bg-health-warning/10 p-2">
				<h4 class="text-sm font-semibold text-health-warning mb-1">Health-Deployment Correlations</h4>
				{#each correlations as corr (corr.serviceName)}
					<p class="text-xs text-subtext-1">
						<span class="font-medium text-text">{corr.serviceName}</span>:
						health changed at {formatTime(corr.healthChangeTime)},
						deployment at {formatTime(corr.deploymentTime)}
					</p>
				{/each}
			</div>
		{/if}

		<!-- Recent Commits -->
		<div>
			<h3 class="text-sm font-semibold text-text mb-2">Recent Commits</h3>
			{#if error}
				<p class="text-sm text-health-error">{error}</p>
			{:else if commits.length === 0}
				<p class="text-sm text-subtext-0">No recent commits</p>
			{:else}
				<div class="flex flex-col gap-1" data-testid="commit-list">
					{#each commits as commit (commit.sha)}
						<div
							class="flex items-center justify-between rounded p-1.5 text-sm {isCorrelatedCommit(commit.timestamp) ? 'border border-health-warning/30 bg-health-warning/10' : 'bg-surface-0/50'}"
							data-testid="commit-row"
						>
							<div class="flex-1 min-w-0">
								<span class="font-mono text-xs text-subtext-0">{commit.sha.slice(0, 7)}</span>
								<span class="ml-2 text-text truncate">{commit.message.split('\n')[0]}</span>
								<span class="ml-2 text-xs text-subtext-0">{commit.author}</span>
								<span class="ml-2 text-xs text-subtext-0">{formatTime(commit.timestamp)}</span>
							</div>
							<button
								class="ml-2 rounded bg-surface-0 px-2 py-0.5 text-xs text-subtext-0 hover:text-text hover:bg-surface-1 shrink-0"
								onclick={() => openRollback(commit)}
								data-testid="rollback-button"
							>
								Rollback
							</button>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Rollback Confirmation Modal (simple inline for now, Story 14.3 adds ConfirmationModal) -->
		{#if rollbackSha && rollbackCommit}
			<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" data-testid="rollback-modal">
				<div class="rounded-lg border border-overlay-0 bg-base p-6 shadow-lg max-w-md w-full">
					<h3 class="text-lg font-semibold text-text mb-3">Confirm Rollback</h3>
					<div class="text-sm text-subtext-1 space-y-2">
						<p><span class="font-medium text-text">SHA:</span> {rollbackCommit.sha}</p>
						<p><span class="font-medium text-text">Message:</span> {rollbackCommit.message.split('\n')[0]}</p>
						<p><span class="font-medium text-text">Author:</span> {rollbackCommit.author}</p>
						<p><span class="font-medium text-text">Time:</span> {formatTime(rollbackCommit.timestamp)}</p>
						<p class="text-health-warning mt-2">
							This will create a revert commit on branch <code class="bg-surface-0 px-1 rounded">{repository.split('/')[1] || 'main'}</code>.
						</p>
					</div>

					{#if rollbackError}
						<p class="mt-3 text-sm text-health-error">{rollbackError}</p>
					{/if}

					<div class="mt-4 flex justify-end gap-2">
						<button
							class="rounded px-3 py-1.5 text-sm text-subtext-0 hover:text-text bg-surface-0 hover:bg-surface-1"
							onclick={closeRollback}
						>
							Cancel
						</button>
						<button
							class="rounded px-3 py-1.5 text-sm text-base bg-health-error hover:bg-health-error/80 disabled:opacity-50"
							onclick={confirmRollback}
							disabled={rollbackLoading}
						>
							{rollbackLoading ? 'Reverting...' : 'Confirm Rollback'}
						</button>
					</div>
				</div>
			</div>
		{/if}
	{/if}
</div>
