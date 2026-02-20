<script lang="ts">
	import {
		getCounts,
		getConnectionStatus,
		getHasProblems,
		getAppVersion
	} from '$lib/serviceStore.svelte';

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

		{#if getConnectionStatus() === 'disconnected'}
			<span class="text-sm font-semibold text-subtext-0 italic">Reconnecting...</span>
		{/if}

		{#if getAppVersion()}
			<span class="text-xs text-subtext-0">Command Center {getAppVersion()}</span>
		{/if}
	</div>
</div>
