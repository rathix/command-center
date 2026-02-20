<script lang="ts">
	import { onMount } from 'svelte';
	import { connect, disconnect } from '$lib/sseClient';
	import StatusBar from '$lib/components/StatusBar.svelte';
	import ServiceList from '$lib/components/ServiceList.svelte';

	onMount(() => {
		connect();
		return () => disconnect();
	});
</script>

<a
	href="#service-list"
	class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded focus:bg-accent-lavender focus:px-4 focus:py-2 focus:text-crust focus:font-semibold"
>
	Skip to service list
</a>

<header class="fixed top-0 right-0 left-0 z-40 bg-mantle px-6 py-4">
	<StatusBar />
</header>

<main id="service-list" tabindex="-1" class="mx-auto mt-16 max-w-[1200px] px-6 py-8">
	<ServiceList />
</main>
