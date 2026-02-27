<script lang="ts">
	import { onDestroy } from 'svelte';
	import type { Service, HealthStatus } from '$lib/types';
	import { formatRelativeTime } from '$lib/formatRelativeTime';
	import TuiDot from './tui/TuiDot.svelte';
	import HoverTooltip from './HoverTooltip.svelte';
	import ServiceIcon from './ServiceIcon.svelte';

	let { service, odd }: { service: Service; odd: boolean } = $props();

	let expanded = $state(false);

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

	const displayStatus = $derived.by(() => service.compositeStatus);

	const responseDisplay = $derived.by(() => {
		if (
			displayStatus === 'unknown' ||
			service.httpCode === null ||
			service.responseTimeMs === null
		) {
			return '—';
		}
		return `${service.httpCode} \u00B7 ${service.responseTimeMs}ms`;
	});

	const responseTextColor = $derived.by(() => responseTextColorMap[displayStatus]);
	const tintColor = $derived.by(() => tintColorMap[displayStatus]);

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

	function toggleExpand(e: MouseEvent | KeyboardEvent) {
		// Don't toggle when clicking the external link — only toggle on row click
		if (e.target instanceof HTMLAnchorElement) return;
		expanded = !expanded;
	}

	function handleRowClick(e: MouseEvent) {
		toggleExpand(e);
	}

	function handleRowKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			toggleExpand(e);
		}
	}

	onDestroy(() => {
		clearHoverTimer();
	});
</script>

<li
	bind:this={rowElement}
	class="relative min-h-[var(--service-row-min-height)] transition-colors duration-300 hover:bg-surface-1 active:scale-[0.99] active:transition-transform active:duration-75 sm:h-auto {odd ? 'bg-surface-0' : ''}"
	style:background-color={tintColor}
	onmouseenter={handleMouseEnter}
	onmousemove={handleMouseMove}
	onmouseleave={handleMouseLeave}
>
	{#if safeHref}
		<div
			class="flex min-h-[var(--touch-target-min)] cursor-pointer items-center gap-2 px-3 sm:gap-3 sm:px-4
				focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent-lavender"
			style="touch-action: manipulation;"
			role="button"
			tabindex="0"
			aria-describedby={tooltipId}
			aria-expanded={expanded}
			onclick={handleRowClick}
			onkeydown={handleRowKeydown}
		>
				<TuiDot status={displayStatus} />
			<ServiceIcon name={iconName} />
			<span class="truncate text-xs font-medium text-text sm:text-sm">{service.displayName}</span>
			<span class="hidden text-xs text-subtext-1 sm:inline">{service.url}</span>
			{#if service.source === 'kubernetes'}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">⎈</span>
			{:else if service.source === 'config'}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">⌂</span>
			{/if}
			{#if service.authGuarded}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">🛡</span>
			{/if}
			<span class="ml-auto hidden text-[11px] sm:inline {responseTextColor}">{responseDisplay}</span>
		</div>
	{:else}
		<div
			aria-describedby={tooltipId}
			class="flex min-h-[var(--touch-target-min)] cursor-pointer items-center gap-2 px-3 opacity-70 sm:gap-3 sm:px-4"
			title="Invalid URL"
			style="touch-action: manipulation;"
			role="button"
			tabindex="0"
			aria-expanded={expanded}
			onclick={handleRowClick}
			onkeydown={handleRowKeydown}
		>
				<TuiDot status={displayStatus} />
			<ServiceIcon name={iconName} />
			<span class="truncate text-xs font-medium text-text sm:text-sm">{service.displayName}</span>
			<span class="hidden text-xs text-subtext-1 sm:inline">{service.url} (invalid)</span>
			{#if service.source === 'kubernetes'}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">⎈</span>
			{:else if service.source === 'config'}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">⌂</span>
			{/if}
			{#if service.authGuarded}
				<span class="text-subtext-0 text-[11px]" aria-hidden="true">🛡</span>
			{/if}
			<span class="ml-auto hidden text-[11px] sm:inline {responseTextColor}">{responseDisplay}</span>
		</div>
	{/if}

	{#if expanded}
		<div
			class="border-t border-surface-1 bg-surface-0 px-4 py-3 text-xs"
			data-testid="expanded-details"
		>
			<div class="grid gap-1">
				<div class="flex gap-2">
					<span class="text-subtext-0">URL:</span>
					{#if safeHref}
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
						<a href={safeHref} target="_blank" rel="noopener noreferrer" class="text-accent-blue underline break-all">{service.url}</a>
					{:else}
						<span class="text-text break-all">{service.url}</span>
					{/if}
				</div>
				{#if service.httpCode !== null}
					<div class="flex gap-2">
						<span class="text-subtext-0">HTTP Code:</span>
						<span class="text-text">{service.httpCode}</span>
					</div>
				{/if}
				{#if service.responseTimeMs !== null}
					<div class="flex gap-2">
						<span class="text-subtext-0">Response Time:</span>
						<span class="text-text">{service.responseTimeMs}ms</span>
					</div>
				{/if}
				{#if service.lastChecked}
					<div class="flex gap-2">
						<span class="text-subtext-0">Last Checked:</span>
						<span class="text-text">{formatRelativeTime(service.lastChecked)}</span>
					</div>
				{/if}
				{#if service.lastStateChange}
					<div class="flex gap-2">
						<span class="text-subtext-0">State Changed:</span>
						<span class="text-text">{formatRelativeTime(service.lastStateChange)}</span>
					</div>
				{/if}
				{#if service.errorSnippet}
					<div class="flex gap-2">
						<span class="text-subtext-0">Error:</span>
						<span class="text-health-error break-all">{service.errorSnippet}</span>
					</div>
				{/if}
				{#if service.podDiagnostic}
					<div class="flex gap-2">
						<span class="text-subtext-0">Pod:</span>
						<span class="text-text">
							{#if service.podDiagnostic.reason}
								{service.podDiagnostic.reason} ·
							{/if}
							{service.podDiagnostic.restartCount} restarts
						</span>
					</div>
				{/if}
			</div>
		</div>
	{/if}
	<HoverTooltip {service} visible={showTooltip} position={tooltipPosition} left={mouseX} id={tooltipId} />
</li>
