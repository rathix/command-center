<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { onMount } from 'svelte';

	let { children } = $props();

	onMount(async () => {
		if ('serviceWorker' in navigator) {
			try {
				const { registerSW } = await import('virtual:pwa-register');
				registerSW({ immediate: true });
			} catch {
				// PWA registration not available in dev/test
			}
		}
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}
