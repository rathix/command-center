<script lang="ts">
	import { fade } from 'svelte/transition';

	let { name }: { name: string } = $props();

	const CDN_BASE = 'https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/svg';
	const src = $derived(`${CDN_BASE}/${encodeURIComponent(name)}.svg`);

	let loaded = $state(false);
	let errored = $state(false);
</script>

{#if errored}
	<span
		transition:fade={{ duration: 200 }}
		class="inline-flex h-4 w-4 shrink-0 items-center justify-center text-[10px] text-subtext-0"
		aria-hidden="true"
	>
		>_
	</span>
{:else}
	<img
		{src}
		alt=""
		width="16"
		height="16"
		loading="lazy"
		aria-hidden="true"
		class="h-4 w-4 shrink-0 transition-opacity duration-200"
		style:opacity={loaded ? 1 : 0}
		onload={() => {
			loaded = true;
		}}
		onerror={() => {
			errored = true;
		}}
	/>
{/if}
