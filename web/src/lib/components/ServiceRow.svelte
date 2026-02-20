<script lang="ts">
	import type { Service, HealthStatus } from '$lib/types';
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

	const responseTextColorMap: Record<HealthStatus, string> = {
		healthy: 'text-subtext-0',
		unhealthy: 'text-health-error',
		authBlocked: 'text-health-auth-blocked',
		unknown: 'text-health-unknown'
	};

	const tintColorMap: Record<HealthStatus, string | undefined> = {
		healthy: undefined,
		unhealthy: 'rgba(243, 139, 168, 0.05)',
		authBlocked: 'rgba(249, 226, 175, 0.03)',
		unknown: undefined
	};

	const responseDisplay = $derived.by(() => {
		if (
			service.status === 'unknown' ||
			service.httpCode === null ||
			service.responseTimeMs === null
		) {
			return '—';
		}
		return `${service.httpCode} \u00B7 ${service.responseTimeMs}ms`;
	});

	const responseTextColor = $derived.by(() => responseTextColorMap[service.status]);
	const tintColor = $derived.by(() => tintColorMap[service.status]);
</script>

<li
	class="h-[46px] transition-colors duration-300 hover:bg-surface-1 {odd ? 'bg-surface-0' : ''}"
	style:background-color={tintColor}
>
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
		<span class="ml-auto text-[11px] {responseTextColor}">{responseDisplay}</span>
	</a>
</li>
