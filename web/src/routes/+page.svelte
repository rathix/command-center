<script lang="ts">
	import { onMount } from 'svelte';
	import { connect, disconnect } from '$lib/sseClient';
	import { initBreakpointListener, destroyBreakpointListener } from '$lib/breakpointStore.svelte';
	import { initConnectivityListener, destroyConnectivityListener, setLastSyncTime } from '$lib/connectivityStore.svelte';
	import { loadLastKnownState, getLastSyncTime } from '$lib/offlineCache';
	import { replaceAll, setConnectionStatus } from '$lib/serviceStore.svelte';
	import StatusBar from '$lib/components/StatusBar.svelte';
	import StalenessIndicator from '$lib/components/StalenessIndicator.svelte';
	import GroupedServiceList from '$lib/components/GroupedServiceList.svelte';

	onMount(() => {
		initBreakpointListener();
		initConnectivityListener();

		if (navigator.onLine) {
			connect();
		} else {
			// Load cached state for offline access
			setConnectionStatus('disconnected');
			loadLastKnownState().then(async (cached) => {
				if (cached) {
					replaceAll(cached, '');
					const syncTime = await getLastSyncTime();
					if (syncTime) {
						setLastSyncTime(syncTime);
					}
				}
			});
		}

		return () => {
			disconnect();
			destroyBreakpointListener();
			destroyConnectivityListener();
		};
	});
</script>

<a
	href="#service-list"
	class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded focus:bg-accent-lavender focus:px-4 focus:py-2 focus:text-crust focus:font-semibold"
>
	Skip to service list
</a>

<header class="fixed top-0 right-0 left-0 z-40 bg-mantle px-3 py-4 sm:px-6">
	<StatusBar />
</header>

<div class="mt-16">
	<StalenessIndicator />
</div>

<main
	id="service-list"
	tabindex="-1"
	class="mx-auto px-3 py-4 sm:px-4 sm:py-6 lg:max-w-[1200px] lg:px-6 lg:py-8"
>
	<GroupedServiceList />
</main>
