<script lang="ts">
	import {
		getCounts,
		getConnectionStatus,
		getHasProblems,
		getAppVersion,
		getLastUpdated,
		getHealthCheckIntervalMs
	} from '$lib/serviceStore.svelte';
	import { formatRelativeTime } from '$lib/formatRelativeTime';

	const segments = $derived.by(() => {
		const c = getCounts();
		const parts: { label: string; count: number; color: string }[] = [];
		if (c.unhealthy > 0)
			parts.push({ label: 'unhealthy', count: c.unhealthy, color: 'text-health-error' });
		if (c.authBlocked > 0)
			parts.push({ label: 'auth-blocked', count: c.authBlocked, color: 'text-health-auth-blocked' });
		if (c.unknown > 0)
			parts.push({ label: 'unknown', count: c.unknown, color: 'text-health-unknown' });
		if (c.healthy > 0) parts.push({ label: 'healthy', count: c.healthy, color: 'text-health-ok' });
		return parts;
	});

	let now = $state(Date.now());

	$effect(() => {
		const id = setInterval(() => {
			now = Date.now();
		}, 1000);
		return () => clearInterval(id);
	});

	const lastUpdatedIso = $derived.by(() => getLastUpdated()?.toISOString() ?? null);
	const lastUpdatedLabel = $derived.by(() => {
		void now;
		return lastUpdatedIso ? formatRelativeTime(lastUpdatedIso) : null;
	});

	const dataAgeMs = $derived.by(() => {
		const lu = getLastUpdated();
		return lu ? now - lu.getTime() : null;
	});

	const stalenessLevel = $derived.by((): 'fresh' | 'aging' | 'stale' => {
		if (getConnectionStatus() === 'disconnected') return 'stale';
		if (dataAgeMs === null) return 'fresh';
		const interval = getHealthCheckIntervalMs();
		if (dataAgeMs > 5 * interval) return 'stale';
		if (dataAgeMs > 2 * interval) return 'aging';
		return 'fresh';
	});

	const stalenessColor = $derived.by(() => {
		switch (stalenessLevel) {
			case 'stale':
				return 'var(--color-health-error)';
			case 'aging':
				return 'var(--color-health-auth-blocked)';
			default:
				return 'var(--color-subtext-0)';
		}
	});

	const connectionStatus = $derived(getConnectionStatus());
	const timestampText = $derived.by(() => {
		if (!lastUpdatedLabel) return null;
		const base = `Last updated ${lastUpdatedLabel}`;
		if (connectionStatus === 'disconnected' && stalenessLevel === 'stale') {
			return `${base} — connection lost`;
		}
		return base;
	});
</script>

<div class="mx-auto max-w-[1200px]">
	<div class="flex items-center justify-between" role="status" aria-live="polite">
		<div class="flex items-center gap-2">
			{#if getConnectionStatus() === 'connecting'}
				<span class="text-sm font-semibold text-subtext-0">Discovering services...</span>
			{:else if getCounts().total === 0 && getConnectionStatus() === 'connected'}
				<span class="text-sm font-semibold text-subtext-0">No services discovered</span>
			{:else if !getHasProblems()}
				<span class="text-sm font-semibold text-health-ok"
					>{getCounts().total} services — all healthy</span
				>
			{:else}
				<span class="text-sm font-semibold">
					{#each segments as segment, i (segment.label)}
						{#if i > 0}<span class="text-subtext-0"> · </span>{/if}
						<span class={segment.color}>{segment.count} {segment.label}</span>
					{/each}
				</span>
			{/if}
		</div>

		<div class="flex items-center gap-3">
			{#if getConnectionStatus() === 'reconnecting'}
				<span class="text-sm font-semibold text-subtext-0 italic">Reconnecting...</span>
			{:else if getConnectionStatus() === 'disconnected'}
				<span class="text-sm font-semibold text-health-error">Connection lost</span>
			{:else if getAppVersion()}
				<span class="text-xs text-subtext-0">Command Center {getAppVersion()}</span>
			{/if}

			{#if lastUpdatedIso && timestampText}
				<time class="text-xs" datetime={lastUpdatedIso} style:color={stalenessColor}>
					{timestampText}
				</time>
			{/if}
		</div>
	</div>
</div>
