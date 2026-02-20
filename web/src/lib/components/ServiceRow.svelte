<script lang="ts">
	import type { Service } from '$lib/types';
	import TuiDot from './tui/TuiDot.svelte';

	let { service, odd }: { service: Service; odd: boolean } = $props();
</script>

<a
	href={service.url}
	target="_blank"
	rel="noopener noreferrer"
	role="listitem"
	class="flex h-[46px] cursor-pointer items-center gap-3 px-4 transition-colors duration-150
		hover:bg-surface-1
		focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent-lavender
		{odd ? 'bg-surface-0' : ''}"
>
	<TuiDot status={service.status} />
	<span class="text-sm font-medium text-text">{service.name}</span>
	<span class="text-xs text-subtext-1">{service.url}</span>
</a>
