<script lang="ts">
	import { onMount } from 'svelte';
	import { connect, disconnect } from '$lib/sseClient';
	import StatusBar from '$lib/components/StatusBar.svelte';
	import TilingLayout from '$lib/components/TilingLayout.svelte';
	import PresetSelector from '$lib/components/PresetSelector.svelte';
	import KeyboardShortcutOverlay from '$lib/components/KeyboardShortcutOverlay.svelte';
	import { getRootNode, loadPresetsFromStorage, loadActiveLayout } from '$lib/layoutStore.svelte';
	import { handleKeyDown } from '$lib/keyboardHandler';
	import { getShowOverlay, hideOverlay } from '$lib/keyboardStore.svelte';

	const root = $derived.by(() => getRootNode());
	const overlayVisible = $derived.by(() => getShowOverlay());

	onMount(() => {
		connect();
		loadPresetsFromStorage();
		loadActiveLayout();

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
		};
	});
</script>

<a
	href="#service-list"
	class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded focus:bg-accent-lavender focus:px-4 focus:py-2 focus:text-crust focus:font-semibold"
>
	Skip to service list
</a>

<header class="fixed top-0 right-0 left-0 z-40 flex items-center justify-between bg-mantle px-6 py-4">
	<StatusBar />
	<PresetSelector />
</header>

<main id="service-list" tabindex="-1" class="fixed top-14 right-0 bottom-0 left-0">
	<TilingLayout node={root} />
</main>

{#if overlayVisible}
	<KeyboardShortcutOverlay visible={overlayVisible} onclose={hideOverlay} />
{/if}
