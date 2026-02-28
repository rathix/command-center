<script lang="ts">
	interface Props {
		title: string;
		message: string;
		details?: Record<string, string>;
		confirmLabel?: string;
		confirmVariant?: 'danger' | 'warning';
		disabled?: boolean;
		onConfirm: () => void;
		onCancel: () => void;
	}

	const {
		title,
		message,
		details = {},
		confirmLabel = 'Confirm',
		confirmVariant = 'danger',
		disabled = false,
		onConfirm,
		onCancel
	}: Props = $props();

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			onCancel();
		}
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			onCancel();
		}
	}

	const confirmColorClass = $derived(
		confirmVariant === 'danger' ? 'bg-red text-crust' : 'bg-yellow text-crust'
	);
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
	class="fixed inset-0 z-50 flex items-center justify-center bg-crust/80"
	onclick={handleBackdropClick}
	role="dialog"
	aria-modal="true"
	aria-labelledby="modal-title"
>
	<div class="w-full max-w-md rounded-lg bg-base p-6 shadow-lg" role="document">
		<h2 id="modal-title" class="mb-2 text-lg font-semibold text-text">{title}</h2>
		<p class="mb-4 text-sm text-subtext-0">{message}</p>

		{#if Object.keys(details).length > 0}
			<div class="mb-4 rounded bg-surface-0 p-3">
				{#each Object.entries(details) as [key, value] (key)}
					<div class="flex justify-between py-1 text-sm">
						<span class="text-subtext-1">{key}</span>
						<span class="font-medium text-text">{value}</span>
					</div>
				{/each}
			</div>
		{/if}

		<div class="flex justify-end gap-3">
			<button
				class="rounded px-4 py-2 text-sm bg-surface-0 text-text hover:bg-surface-1"
				onclick={onCancel}
				type="button"
			>
				Cancel
			</button>
			<button
				class="rounded px-4 py-2 text-sm font-semibold {confirmColorClass} disabled:opacity-50"
				onclick={onConfirm}
				{disabled}
				type="button"
			>
				{confirmLabel}
			</button>
		</div>
	</div>
</div>
