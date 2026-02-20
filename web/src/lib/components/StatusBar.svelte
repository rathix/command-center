<script lang="ts">
	import { getCounts, getConnectionStatus } from '$lib/serviceStore.svelte';
</script>

<div class="mx-auto max-w-[1200px]">
	<div class="flex items-center justify-between" role="status" aria-live="polite">
		<div class="flex items-center gap-2">
			{#if getConnectionStatus() === 'connecting'}
				<span class="text-sm font-semibold text-subtext-0">Discovering services...</span>
			{:else if getCounts().total === 0 && getConnectionStatus() === 'connected'}
				<span class="text-sm font-semibold text-subtext-0">No services discovered</span>
			{:else}
				<span class="text-sm font-semibold text-text">{getCounts().total} services</span>
			{/if}
		</div>

		{#if getConnectionStatus() === 'disconnected'}
			<span class="text-sm font-semibold text-subtext-0 italic">Reconnecting...</span>
		{/if}
	</div>
</div>
