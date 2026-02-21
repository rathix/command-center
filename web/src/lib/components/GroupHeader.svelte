<script lang="ts">
	import type { ServiceGroup } from '$lib/types';
	import { toggleGroupCollapse } from '$lib/serviceStore.svelte';

	let { group, controlsId }: { group: ServiceGroup; controlsId: string } = $props();

	const chevron = $derived(group.expanded ? '▾' : '▸');

	const summaryParts = $derived.by(() => {
		const parts: { count: number; label: string; colorClass: string }[] = [];
		if (group.counts.healthy > 0)
			parts.push({ count: group.counts.healthy, label: 'healthy', colorClass: 'text-health-ok' });
		if (group.counts.unhealthy > 0)
			parts.push({ count: group.counts.unhealthy, label: 'unhealthy', colorClass: 'text-health-error' });
		if (group.counts.authBlocked > 0)
			parts.push({ count: group.counts.authBlocked, label: 'auth-blocked', colorClass: 'text-health-auth-blocked' });
		if (group.counts.unknown > 0)
			parts.push({ count: group.counts.unknown, label: 'unknown', colorClass: 'text-health-unknown' });
		return parts;
	});

	function handleClick() {
		toggleGroupCollapse(group.name);
	}

	function handleKeydown(e: KeyboardEvent) {
		const isEnter = e.key === 'Enter';
		const isSpace = e.key === ' ' || e.key === 'Space' || e.key === 'Spacebar';
		if (isEnter || isSpace) {
			if (isSpace) e.preventDefault();
			toggleGroupCollapse(group.name);
		}
	}
</script>

<div
	class="flex h-[36px] cursor-pointer items-center gap-2 bg-mantle px-4
		focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent-lavender"
	role="button"
	tabindex="0"
	aria-expanded={group.expanded}
	aria-controls={controlsId}
	onclick={handleClick}
	onkeydown={handleKeydown}
>
	<span class="w-3 text-xs text-subtext-0" aria-hidden="true">{chevron}</span>
	<span class="text-sm font-medium text-text">{group.name}</span>
	<span class="text-xs text-subtext-0">
		({#each summaryParts as part, i (part.label)}{#if i > 0}, {/if}<span class={part.colorClass}>{part.count} {part.label}</span>{/each})
	</span>
</div>
