<script lang="ts">
	import { getBindingsList } from '$lib/keyboardStore.svelte';

	let { visible, onclose }: { visible: boolean; onclose: () => void } = $props();

	const bindingsList = $derived.by(() => getBindingsList());

	function handleKeyDown(e: KeyboardEvent) {
		if (e.key === 'Escape' || e.key === '?') {
			e.preventDefault();
			onclose();
		}
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			onclose();
		}
	}
</script>

{#if visible}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-crust/80 backdrop-blur-sm"
		onclick={handleBackdropClick}
		onkeydown={handleKeyDown}
		data-testid="keyboard-overlay"
	>
		<div
			class="max-w-md rounded-lg border border-overlay-0 bg-surface-0 p-6"
			role="dialog"
			aria-label="Keyboard shortcuts"
		>
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-lg font-bold text-accent-lavender">Keyboard Shortcuts</h2>
				<button
					class="text-subtext-0 hover:text-text"
					onclick={onclose}
					aria-label="Close"
					data-testid="overlay-close"
				>
					&times;
				</button>
			</div>
			<div class="space-y-2">
				{#each bindingsList as binding (binding.keyCombo)}
					<div class="flex items-center justify-between gap-6">
						<kbd
							class="rounded bg-surface-1 px-2 py-0.5 text-sm text-text"
							data-testid="kbd"
						>
							{binding.keyCombo}
						</kbd>
						<span class="text-sm text-subtext-1">{binding.description}</span>
					</div>
				{/each}
			</div>
		</div>
	</div>
{/if}
