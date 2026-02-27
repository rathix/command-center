<script lang="ts">
	export interface ContextAction {
		label: string;
		icon?: string;
		action: () => void;
		disabled?: boolean;
	}

	let {
		visible,
		x,
		y,
		actions,
		onClose
	}: {
		visible: boolean;
		x: number;
		y: number;
		actions: ContextAction[];
		onClose: () => void;
	} = $props();

	const clampedX = $derived.by(() => {
		if (typeof window === 'undefined') return x;
		return Math.min(x, window.innerWidth - 200);
	});

	const clampedY = $derived.by(() => {
		if (typeof window === 'undefined') return y;
		const menuHeight = actions.length * 44 + 16;
		return Math.min(y, window.innerHeight - menuHeight);
	});

	function handleAction(action: ContextAction) {
		if (action.disabled) return;
		action.action();
		onClose();
	}

	function handleBackdropClick() {
		onClose();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			onClose();
		}
	}
</script>

{#if visible}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50"
		onclick={handleBackdropClick}
		onkeydown={handleKeydown}
		data-testid="context-menu-backdrop"
	></div>
	<div
		class="fixed z-50 min-w-[180px] rounded-lg border border-overlay-0 bg-surface-0 py-2 shadow-lg"
		style:left="{clampedX}px"
		style:top="{clampedY}px"
		role="menu"
		data-testid="context-menu"
	>
		{#each actions as action (action.label)}
			<button
				class="flex w-full min-h-[var(--touch-target-min)] items-center gap-2 px-4 text-left text-sm text-text hover:bg-surface-1
					{action.disabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'}"
				role="menuitem"
				disabled={action.disabled}
				onclick={() => handleAction(action)}
			>
				{#if action.icon}
					<span aria-hidden="true">{action.icon}</span>
				{/if}
				{action.label}
			</button>
		{/each}
	</div>
{/if}
