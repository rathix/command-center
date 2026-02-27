<script lang="ts">
	import { getIsOnline, getLastSyncTime } from '$lib/connectivityStore.svelte';

	let now = $state(Date.now());

	$effect(() => {
		const id = setInterval(() => {
			now = Date.now();
		}, 30_000);
		return () => clearInterval(id);
	});

	const relativeTime = $derived.by(() => {
		void now;
		const syncTime = getLastSyncTime();
		if (!syncTime) return null;
		const diff = Date.now() - new Date(syncTime).getTime();
		if (diff < 60_000) return 'less than a minute ago';
		if (diff < 3_600_000) {
			const mins = Math.floor(diff / 60_000);
			return `${mins} minute${mins === 1 ? '' : 's'} ago`;
		}
		if (diff < 86_400_000) {
			const hours = Math.floor(diff / 3_600_000);
			return `${hours} hour${hours === 1 ? '' : 's'} ago`;
		}
		const days = Math.floor(diff / 86_400_000);
		return `${days} day${days === 1 ? '' : 's'} ago`;
	});

	const showBanner = $derived.by(() => !getIsOnline() && getLastSyncTime() !== null);
</script>

{#if showBanner}
	<div
		class="bg-surface-0 text-accent-peach px-3 py-2 text-center text-xs font-medium sm:px-6 sm:text-sm"
		role="alert"
		data-testid="staleness-banner"
	>
		Offline — last synced {relativeTime}
	</div>
{/if}
