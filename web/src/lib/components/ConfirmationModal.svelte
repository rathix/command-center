<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		open = false,
		title = 'Confirm',
		loading = false,
		onconfirm,
		oncancel,
		children
	}: {
		open: boolean;
		title: string;
		loading?: boolean;
		onconfirm: () => void;
		oncancel: () => void;
		children: Snippet;
	} = $props();

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && open) {
			event.preventDefault();
			oncancel();
		}
	}

	function handleBackdropClick() {
		oncancel();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
		role="dialog"
		aria-modal="true"
		aria-labelledby="confirmation-title"
		data-testid="confirmation-modal"
	>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="fixed inset-0" onclick={handleBackdropClick}></div>
		<div class="relative z-10 rounded-lg border border-overlay-0 bg-base p-6 shadow-lg max-w-md w-full">
			<h2 id="confirmation-title" class="text-lg font-semibold text-text mb-3">{title}</h2>

			<div class="text-sm text-subtext-1">
				{@render children()}
			</div>

			<div class="mt-4 flex justify-end gap-2">
				<button
					class="rounded px-3 py-1.5 text-sm text-subtext-0 hover:text-text bg-surface-0 hover:bg-surface-1"
					onclick={oncancel}
					data-testid="cancel-button"
				>
					Cancel
				</button>
				<button
					class="rounded px-3 py-1.5 text-sm text-base bg-health-error hover:bg-health-error/80 disabled:opacity-50"
					onclick={onconfirm}
					disabled={loading}
					data-testid="confirm-button"
				>
					{#if loading}
						<span class="inline-flex items-center gap-1">
							<span class="animate-spin text-xs">&#9696;</span>
							Processing...
						</span>
					{:else}
						Confirm
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}
