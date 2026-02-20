<script lang="ts">
	import { getCounts, getConnectionStatus } from '$lib/serviceStore.svelte';
</script>

<div class="flex items-center justify-between" role="status" aria-live="polite">
	{#if getConnectionStatus() === 'connecting'}
		<span class="text-sm font-semibold text-subtext-0">Discovering services...</span>
	{:else if getConnectionStatus() === 'disconnected'}
		<span class="text-sm font-semibold text-subtext-0">Reconnecting...</span>
	{:else if getCounts().total === 0}
		<span class="text-sm font-semibold text-subtext-0">No services discovered</span>
	{:else}
		<span class="text-sm font-semibold text-text">{getCounts().total} services</span>
	{/if}
</div>
