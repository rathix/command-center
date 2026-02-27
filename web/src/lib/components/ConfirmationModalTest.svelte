<script lang="ts">
	import ConfirmationModal from './ConfirmationModal.svelte';

	let {
		open = false,
		title = 'Confirm',
		bodyText = '',
		loading = false
	}: {
		open: boolean;
		title: string;
		bodyText: string;
		loading: boolean;
	} = $props();

	let confirmed = $state(false);
	let cancelled = $state(false);

	function handleConfirm() {
		confirmed = true;
	}

	function handleCancel() {
		cancelled = true;
	}
</script>

<ConfirmationModal {open} {title} {loading} onconfirm={handleConfirm} oncancel={handleCancel}>
	<p>{bodyText}</p>
</ConfirmationModal>

{#if confirmed}
	<span data-testid="confirmed-flag">confirmed</span>
{/if}
{#if cancelled}
	<span data-testid="cancelled-flag">cancelled</span>
{/if}
