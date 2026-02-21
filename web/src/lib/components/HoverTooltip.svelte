<script lang="ts">
	import type { Service, HealthStatus } from '$lib/types';
	import { formatRelativeTime } from '$lib/formatRelativeTime';

	let { service, visible, position, left, id }: {
		service: Service;
		visible: boolean;
		position: 'below' | 'above';
		left: number;
		id: string;
	} = $props();

	const healthColorMap: Record<HealthStatus, string> = {
		healthy: 'text-health-ok',
		unhealthy: 'text-health-error',
		authBlocked: 'text-health-auth-blocked',
		unknown: 'text-health-unknown'
	};

	const statusLabelMap: Record<HealthStatus, string> = {
		healthy: 'healthy for',
		unhealthy: 'unhealthy for',
		authBlocked: 'auth-blocked for',
		unknown: 'unknown for'
	};

	let now = $state(Date.now());

	$effect(() => {
		if (!visible) return;
		const id = setInterval(() => {
			now = Date.now();
		}, 1000);
		return () => clearInterval(id);
	});

	const stateDisplay = $derived.by(() => {
		void now; // force re-evaluation each tick
		const label = statusLabelMap[service.status];
		const isUnhealthy = service.status === 'unhealthy';
		// Unhealthy uses precise (H M S), others use simple (H or M or S)
		const time = formatRelativeTime(service.lastStateChange, false, isUnhealthy);
		return `${label} ${time}`;
	});

	const stateColor = $derived.by(() => healthColorMap[service.status]);

	const errorLine = $derived.by(() => {
		if (service.status !== 'unhealthy' || !service.errorSnippet) return null;
		const snippet = service.errorSnippet;
		return snippet.length > 80 ? snippet.slice(0, 80) + '…' : snippet;
	});

	const sourceLine = $derived.by(() => {
		if (service.source === 'kubernetes') return `Source: Kubernetes / ${service.namespace}`;
		if (service.source === 'config') return 'Source: Custom config';
		return null;
	});

	const positionClasses = $derived.by(() => {
		return position === 'above' ? 'bottom-full mb-1' : 'top-full mt-1';
	});

	let tooltipElement: HTMLDivElement | undefined = $state(undefined);
	let adjustedLeft = $state(0);

	$effect(() => {
		if (visible && tooltipElement) {
			const rect = tooltipElement.getBoundingClientRect();
			const overflow = rect.right - window.innerWidth;
			if (overflow > 0) {
				adjustedLeft = left - overflow - 8; // 8px padding from edge
			} else {
				adjustedLeft = left;
			}
		}
	});
</script>

{#if visible}
	<div
		{id}
		bind:this={tooltipElement}
		role="tooltip"
		class="absolute z-50 bg-surface-1 border border-overlay-1 rounded-sm p-2 text-[11px] text-subtext-0 max-w-[400px] {positionClasses}"
		style:left="{adjustedLeft}px"
	>
		<div class={stateColor}>{stateDisplay}</div>
		{#if errorLine}
			<div class="truncate">{errorLine}</div>
		{/if}
		{#if sourceLine}
			<div>{sourceLine}</div>
		{/if}
	</div>
{/if}
