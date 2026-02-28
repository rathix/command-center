<script lang="ts">
	import {
		getPresets,
		getActivePresetName,
		savePreset,
		restorePreset,
		deletePreset
	} from '$lib/layoutStore.svelte';

	const presets = $derived.by(() => getPresets());
	const activePresetName = $derived.by(() => getActivePresetName());

	let showDropdown = $state(false);
	let showSaveInput = $state(false);
	let presetName = $state('');

	function handleToggle(e: MouseEvent) {
		e.stopPropagation();
		showDropdown = !showDropdown;
		showSaveInput = false;
		presetName = '';
	}

	function handleRestore(name: string) {
		restorePreset(name);
		showDropdown = false;
	}

	function handleDelete(e: MouseEvent, name: string) {
		e.stopPropagation();
		deletePreset(name);
	}

	function handleSaveClick(e: MouseEvent) {
		e.stopPropagation();
		showSaveInput = true;
	}

	function handleSaveSubmit() {
		const trimmed = presetName.trim();
		if (trimmed) {
			savePreset(trimmed);
			presetName = '';
			showSaveInput = false;
			showDropdown = false;
		}
	}

	function handleSaveKeyDown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			handleSaveSubmit();
		} else if (e.key === 'Escape') {
			showSaveInput = false;
		}
	}

	function handleClickOutside() {
		if (showDropdown) {
			showDropdown = false;
			showSaveInput = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutside} />

<div class="relative" data-testid="preset-selector">
	<button
		class="rounded border border-overlay-0 bg-surface-0 px-3 py-1 text-xs text-subtext-1 hover:bg-surface-1"
		onclick={handleToggle}
		data-testid="preset-toggle"
	>
		{activePresetName ?? 'Custom'}
	</button>

	{#if showDropdown}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div
			class="absolute right-0 top-full z-50 mt-1 min-w-[180px] rounded-lg border border-overlay-0 bg-surface-0 shadow-lg"
			onclick={(e) => e.stopPropagation()}
			data-testid="preset-dropdown"
		>
			{#each [...presets.entries()] as [name] (name)}
				<div
					class="flex items-center justify-between px-3 py-2 hover:bg-surface-1"
				>
					<button
						class="flex-1 text-left text-xs {name === activePresetName
							? 'font-bold text-accent-lavender'
							: 'text-subtext-1'} cursor-pointer"
						onclick={() => handleRestore(name)}
						data-testid={`preset-item-${name}`}
					>
						{name}
					</button>
					<button
						class="ml-2 text-overlay-1 hover:text-health-error"
						onclick={(e) => handleDelete(e, name)}
						aria-label="Delete preset {name}"
						data-testid={`preset-delete-${name}`}
					>
						&times;
					</button>
				</div>
			{/each}

			<div class="border-t border-overlay-0 px-3 py-2">
				{#if showSaveInput}
					<div class="flex gap-2">
						<input
							type="text"
							bind:value={presetName}
							placeholder="Preset name..."
							class="flex-1 rounded border border-overlay-0 bg-base px-2 py-1 text-xs text-text"
							onkeydown={handleSaveKeyDown}
							data-testid="preset-name-input"
						/>
						<button
							class="rounded bg-accent-lavender px-3 py-1 text-xs font-semibold text-crust"
							onclick={handleSaveSubmit}
							data-testid="preset-save-confirm"
						>
							Save
						</button>
					</div>
				{:else}
					<button
						class="w-full rounded bg-accent-lavender px-3 py-1 text-xs font-semibold text-crust"
						onclick={handleSaveClick}
						data-testid="preset-save-button"
					>
						Save Layout
					</button>
				{/if}
			</div>
		</div>
	{/if}
</div>
