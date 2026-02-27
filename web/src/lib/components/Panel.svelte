<script lang="ts">
	import type { LeafNode } from '$lib/types';
	import {
		getActivePanelId,
		setActivePanel,
		splitPanel,
		closePanel,
		getIsLastPanel,
		AVAILABLE_PANEL_TYPES
	} from '$lib/layoutStore.svelte';
	import GroupedServiceList from './GroupedServiceList.svelte';
	import ContentSelector from './ContentSelector.svelte';

	let { node }: { node: LeafNode } = $props();

	const isActive = $derived.by(() => node.panelId === getActivePanelId());
	const isLast = $derived.by(() => getIsLastPanel());

	let showSplitMenu = $state(false);

	function handlePanelClick() {
		setActivePanel(node.panelId);
	}

	function handleSplit(direction: 'horizontal' | 'vertical') {
		splitPanel(node.panelId, direction);
		showSplitMenu = false;
	}

	function handleClose() {
		closePanel(node.panelId);
	}

	function handleSplitToggle(e: MouseEvent) {
		e.stopPropagation();
		showSplitMenu = !showSplitMenu;
	}

	function handleClickOutside() {
		if (showSplitMenu) showSplitMenu = false;
	}

	function capitalize(s: string): string {
		return s.charAt(0).toUpperCase() + s.slice(1);
	}

	const isAvailable = $derived.by(() => AVAILABLE_PANEL_TYPES.has(node.panelType));
</script>

<svelte:window onclick={handleClickOutside} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="flex flex-col overflow-hidden transition-colors duration-150 {isActive
		? 'border-2 border-[var(--color-panel-active)]'
		: 'border border-[var(--color-panel-border)]'}"
	onclick={handlePanelClick}
	data-testid="panel"
	data-panel-id={node.panelId}
>
	<div
		class="flex h-8 shrink-0 items-center justify-between bg-mantle px-3"
		data-testid="panel-header"
	>
		<span class="text-xs text-subtext-1"
			>{capitalize(node.panelType)}{#if !isAvailable}
				<span class="text-overlay-0"> (unavailable)</span>
			{/if}</span
		>
		<div class="flex items-center gap-1">
			<div class="relative">
				<button
					class="text-subtext-0 hover:text-text"
					style="font-size: 16px; line-height: 1;"
					onclick={handleSplitToggle}
					aria-label="Split panel"
					data-testid="split-button"
				>
					&#8862;
				</button>
				{#if showSplitMenu}
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<div
						class="absolute right-0 top-full z-50 mt-1 rounded border border-overlay-0 bg-surface-0 shadow-lg"
						onclick={(e) => e.stopPropagation()}
						data-testid="split-menu"
					>
						<button
							class="block w-full whitespace-nowrap px-3 py-1.5 text-left text-xs text-subtext-1 hover:bg-surface-1"
							onclick={() => handleSplit('horizontal')}
							data-testid="split-horizontal"
						>
							Split Horizontal
						</button>
						<button
							class="block w-full whitespace-nowrap px-3 py-1.5 text-left text-xs text-subtext-1 hover:bg-surface-1"
							onclick={() => handleSplit('vertical')}
							data-testid="split-vertical"
						>
							Split Vertical
						</button>
					</div>
				{/if}
			</div>
			{#if !isLast}
				<button
					class="text-subtext-0 hover:text-health-error"
					style="font-size: 16px; line-height: 1;"
					onclick={handleClose}
					aria-label="Close panel"
					data-testid="close-button"
				>
					&times;
				</button>
			{/if}
		</div>
	</div>
	<div class="flex-1 overflow-auto">
		{#if node.panelType === 'services' && isAvailable}
			<GroupedServiceList />
		{:else if !isAvailable}
			<ContentSelector panelId={node.panelId} />
		{:else}
			<ContentSelector panelId={node.panelId} />
		{/if}
	</div>
</div>
