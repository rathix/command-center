<script lang="ts">
	import type { Service } from '$lib/types';
	import TuiDot from './tui/TuiDot.svelte';

	let { service, odd }: { service: Service; odd: boolean } = $props();

	function sanitizeServiceUrl(url: string): string | null {
		try {
			const parsed = new URL(url);
			if (parsed.protocol === 'http:' || parsed.protocol === 'https:') {
				return parsed.toString();
			}
		} catch {
			// Invalid URL should not be navigable.
		}

		return null;
	}

	const safeHref = $derived.by(() => sanitizeServiceUrl(service.url));

	function preventUnsafeNavigation(event: MouseEvent): void {
		if (!safeHref) {
			event.preventDefault();
		}
	}
</script>

<li class="h-[46px] transition-colors duration-150 hover:bg-surface-1 {odd ? 'bg-surface-0' : ''}">
	<a
		href={safeHref ?? '#'}
		target="_blank"
		rel="noopener noreferrer"
		aria-disabled={!safeHref}
		class="flex h-full cursor-pointer items-center gap-3 px-4
			focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent-lavender"
		onclick={preventUnsafeNavigation}
	>
		<TuiDot status={service.status} />
		<span class="text-sm font-medium text-text">{service.displayName}</span>
		<span class="text-xs text-subtext-1">{service.url}</span>
	</a>
</li>
