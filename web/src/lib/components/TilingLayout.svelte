<script lang="ts">
	import type { LayoutNode } from '$lib/types';
	import { resizePanel } from '$lib/layoutStore.svelte';
	import Panel from './Panel.svelte';
	import TilingLayout from './TilingLayout.svelte';

	let { node }: { node: LayoutNode } = $props();

	let isDragging = $state(false);
	let containerEl: HTMLDivElement | undefined = $state();

	function handleGutterPointerDown(e: PointerEvent) {
		e.preventDefault();
		const target = e.currentTarget as HTMLElement;
		target.setPointerCapture(e.pointerId);
		isDragging = true;

		function handlePointerMove(moveEvt: PointerEvent) {
			if (node.type !== 'branch' || !containerEl) return;

			const rect = containerEl.getBoundingClientRect();
			let ratio: number;

			if (node.direction === 'horizontal') {
				ratio = (moveEvt.clientX - rect.left) / rect.width;
			} else {
				ratio = (moveEvt.clientY - rect.top) / rect.height;
			}

			// Find first leaf in first child for resizePanel call
			const firstLeafId = findFirstLeafId(node.first);
			if (firstLeafId) {
				resizePanel(firstLeafId, ratio);
			}
		}

		function handlePointerUp() {
			isDragging = false;
			target.releasePointerCapture(e.pointerId);
			window.removeEventListener('pointermove', handlePointerMove);
			window.removeEventListener('pointerup', handlePointerUp);
		}

		window.addEventListener('pointermove', handlePointerMove);
		window.addEventListener('pointerup', handlePointerUp);
	}

	function findFirstLeafId(n: LayoutNode): string | null {
		if (n.type === 'leaf') return n.panelId;
		return findFirstLeafId(n.first);
	}
</script>

{#if node.type === 'leaf'}
	<div class="h-full w-full min-h-0 min-w-0">
		<Panel {node} />
	</div>
{:else}
	<div
		class="flex h-full w-full min-h-0 min-w-0 {node.direction === 'horizontal'
			? 'flex-row'
			: 'flex-col'}"
		bind:this={containerEl}
		data-testid="branch"
	>
		<div
			style="flex: 0 0 {node.ratio * 100}%;"
			class="min-h-0 min-w-0 overflow-hidden"
		>
			<TilingLayout node={node.first} />
		</div>
		<div
			class="shrink-0 bg-[var(--color-gutter)] transition-colors hover:bg-[var(--color-gutter-hover)] {node.direction ===
			'horizontal'
				? 'w-1 cursor-col-resize px-[2px]'
				: 'h-1 cursor-row-resize py-[2px]'} {isDragging
				? 'bg-[var(--color-gutter-hover)]'
				: ''}"
			onpointerdown={handleGutterPointerDown}
			role="separator"
			data-testid="gutter"
		></div>
		<div
			style="flex: 0 0 {(1 - node.ratio) * 100}%;"
			class="min-h-0 min-w-0 overflow-hidden"
		>
			<TilingLayout node={node.second} />
		</div>
	</div>
{/if}
