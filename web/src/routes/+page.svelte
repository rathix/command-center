<script lang="ts">
	import { onMount } from 'svelte';
	import { connect, disconnect } from '$lib/sseClient';
	import { initBreakpointListener, destroyBreakpointListener } from '$lib/breakpointStore.svelte';
	import { initConnectivityListener, destroyConnectivityListener, setLastSyncTime } from '$lib/connectivityStore.svelte';
	import { loadLastKnownState, getLastSyncTime } from '$lib/offlineCache';
	import { replaceAll, setConnectionStatus } from '$lib/serviceStore.svelte';
	import StatusBar from '$lib/components/StatusBar.svelte';
	import TilingLayout from '$lib/components/TilingLayout.svelte';
	import PresetSelector from '$lib/components/PresetSelector.svelte';
	import KeyboardShortcutOverlay from '$lib/components/KeyboardShortcutOverlay.svelte';
	import StalenessIndicator from '$lib/components/StalenessIndicator.svelte';
	import { getRootNode, loadPresetsFromStorage, loadActiveLayout } from '$lib/layoutStore.svelte';
	import { handleKeyDown } from '$lib/keyboardHandler';
	import { getShowOverlay, hideOverlay } from '$lib/keyboardStore.svelte';

	const root = $derived.by(() => getRootNode());
	const overlayVisible = $derived.by(() => getShowOverlay());

	onMount(() => {
		initBreakpointListener();
		initConnectivityListener();
		loadPresetsFromStorage();
		loadActiveLayout();

		if (navigator.onLine) {
			connect();
		} else {
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

		function onKeyDown(e: KeyboardEvent) {
			if (
				e.target instanceof HTMLInputElement ||
				e.target instanceof HTMLTextAreaElement
			) {
				return;
			}
			handleKeyDown(e);
		}
		window.addEventListener('keydown', onKeyDown);

		return () => {
			disconnect();
			window.removeEventListener('keydown', onKeyDown);
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

<header class="fixed top-0 right-0 left-0 z-40 flex items-center justify-between bg-mantle px-3 py-4 sm:px-6">
	<StatusBar />
	<PresetSelector />
</header>

<div class="mt-16">
	<StalenessIndicator />
</div>

<main id="service-list" tabindex="-1" class="fixed top-14 right-0 bottom-0 left-0">
	<TilingLayout node={root} />
</main>

{#if overlayVisible}
	<KeyboardShortcutOverlay visible={overlayVisible} onclose={hideOverlay} />
{/if}
