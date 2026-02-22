<script lang="ts">
	import { onDestroy } from 'svelte';
	import type { Service, HealthStatus } from '$lib/types';
	import TuiDot from './tui/TuiDot.svelte';
	import HoverTooltip from './HoverTooltip.svelte';
	import ServiceIcon from './ServiceIcon.svelte';

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
	const iconName = $derived.by(() => {
		const icon = service.icon?.trim();
		return icon || service.displayName;
	});

	const responseTextColorMap: Record<HealthStatus, string> = {
		healthy: 'text-subtext-0',
		degraded: 'text-health-degraded',
		unhealthy: 'text-health-error',
		unknown: 'text-health-unknown'
	};

	const tintColorMap: Record<HealthStatus, string | undefined> = {
		healthy: undefined,
		degraded: 'rgba(249, 226, 175, 0.03)',
		unhealthy: 'rgba(243, 139, 168, 0.05)',
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

	const tooltipId = $derived.by(() => `tooltip-${service.namespace}-${service.name}`);

	let rowElement: HTMLLIElement | undefined = $state(undefined);
	let hoverTimer: ReturnType<typeof setTimeout> | null = null;
	let hovered = $state(false);
	let showTooltip = $state(false);
	let mouseX = $state(0);
	let tooltipPosition: 'below' | 'above' = $state('below');

	function clearHoverTimer() {
		if (hoverTimer) {
			clearTimeout(hoverTimer);
			hoverTimer = null;
		}
	}

	const TOOLTIP_FLIP_THRESHOLD = 200;

	function getTooltipPosition(): 'below' | 'above' {
		if (!rowElement) return 'below';
		const rect = rowElement.getBoundingClientRect();
		const spaceBelow = window.innerHeight - rect.bottom;
		return spaceBelow < TOOLTIP_FLIP_THRESHOLD ? 'above' : 'below';
	}

	function handleMouseEnter(e: MouseEvent) {
		hovered = true;
		clearHoverTimer();

		if (rowElement) {
			const rect = rowElement.getBoundingClientRect();
			mouseX = e.clientX - rect.left;
		}

		hoverTimer = setTimeout(() => {
			if (!hovered) return;
			tooltipPosition = getTooltipPosition();
			showTooltip = true;
			hoverTimer = null;
		}, 200);
	}

	function handleMouseMove(e: MouseEvent) {
		if (hovered && !showTooltip && rowElement) {
			const rect = rowElement.getBoundingClientRect();
			mouseX = e.clientX - rect.left;
		}
	}

	function handleMouseLeave() {
		hovered = false;
		showTooltip = false;
		clearHoverTimer();
	}

	onDestroy(() => {
		clearHoverTimer();
	});
</script>

<li
	bind:this={rowElement}
	class="relative h-[46px] transition-colors duration-300 hover:bg-surface-1 {odd ? 'bg-surface-0' : ''}"
	style:background-color={tintColor}
	onmouseenter={handleMouseEnter}
	onmousemove={handleMouseMove}
	onmouseleave={handleMouseLeave}
>
	{#if safeHref}
		<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
		<a
			href={safeHref}
			target="_blank"
			rel="noopener noreferrer"
			aria-describedby={tooltipId}
			class="flex h-full cursor-pointer items-center gap-3 px-4
				focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent-lavender"
		>
			<TuiDot status={service.status} />
			<ServiceIcon name={iconName} />
			<span class="text-sm font-medium text-text">{service.displayName}</span>
			<span class="text-xs text-subtext-1">{service.url}</span>
			{#if service.source === 'kubernetes'}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">⎈</span>
			{:else if service.source === 'config'}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">⌂</span>
			{/if}
			{#if service.authGuarded}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">🛡</span>
			{/if}
			<span class="ml-auto text-[11px] {responseTextColor}">{responseDisplay}</span>
		</a>
	{:else}
		<div
			aria-describedby={tooltipId}
			class="flex h-full items-center gap-3 px-4 opacity-70"
			title="Invalid URL"
		>
			<TuiDot status={service.status} />
			<ServiceIcon name={iconName} />
			<span class="text-sm font-medium text-text">{service.displayName}</span>
			<span class="text-xs text-subtext-1">{service.url} (invalid)</span>
			{#if service.source === 'kubernetes'}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">⎈</span>
			{:else if service.source === 'config'}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">⌂</span>
			{/if}
			{#if service.authGuarded}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">🛡</span>
			{/if}
			<span class="ml-auto text-[11px] {responseTextColor}">{responseDisplay}</span>
		</div>
	{/if}
	<HoverTooltip {service} visible={showTooltip} position={tooltipPosition} left={mouseX} id={tooltipId} />
</li>
