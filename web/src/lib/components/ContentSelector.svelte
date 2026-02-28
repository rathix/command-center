<script lang="ts">
	import type { PanelType } from '$lib/types';
	import { setPanelType, AVAILABLE_PANEL_TYPES } from '$lib/layoutStore.svelte';

	let { panelId }: { panelId: string } = $props();

	const options: { type: PanelType; label: string; icon: string }[] = [
		{ type: 'services', label: 'Services', icon: '◆' },
		{ type: 'terminal', label: 'Terminal', icon: '▸' },
		{ type: 'logs', label: 'Logs', icon: '☰' },
		{ type: 'nodes', label: 'Nodes', icon: '⬡' },
		{ type: 'gitops', label: 'GitOps', icon: '⎇' }
	];

	function handleSelect(type: PanelType) {
		if (AVAILABLE_PANEL_TYPES.has(type)) {
			setPanelType(panelId, type);
		}
	}
</script>

<div class="flex h-full items-center justify-center bg-base p-6" data-testid="content-selector">
	<div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
		{#each options as option (option.type)}
			{@const available = AVAILABLE_PANEL_TYPES.has(option.type)}
			<button
				class="flex flex-col items-center gap-2 rounded border px-6 py-4 text-sm transition-colors
					{available
					? 'border-overlay-0 bg-surface-0 text-subtext-1 hover:bg-surface-1 cursor-pointer'
					: 'border-overlay-0 bg-surface-0 text-overlay-0 cursor-not-allowed opacity-50'}"
				onclick={() => handleSelect(option.type)}
				disabled={!available}
				data-testid={`content-option-${option.type}`}
			>
				<span class="text-lg">{option.icon}</span>
				<span>{option.label}</span>
				{#if !available}
					<span class="text-[10px] text-overlay-0">coming soon</span>
				{/if}
			</button>
		{/each}
	</div>
</div>
