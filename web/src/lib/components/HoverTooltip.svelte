<script lang="ts">
	import type { Service, HealthStatus } from '$lib/types';
	import { formatRelativeTime } from '$lib/formatRelativeTime';

	let { service, visible, position, id }: {
		service: Service;
		visible: boolean;
		position: 'below' | 'above';
		id: string;
	} = $props();

	const healthColorMap: Record<HealthStatus, string> = {
		healthy: 'text-health-ok',
		unhealthy: 'text-health-error',
		authBlocked: 'text-health-auth-blocked',
		unknown: 'text-health-unknown'
	};

	const statusLabelMap: Record<HealthStatus, string> = {
		healthy: 'healthy since',
		unhealthy: 'unhealthy for',
		authBlocked: 'auth-blocked for',
		unknown: 'unknown since'
	};

	function formatUnhealthyDuration(isoTimestamp: string | null): string {
		if (!isoTimestamp) return 'unknown';

		const then = new Date(isoTimestamp).getTime();
		if (Number.isNaN(then)) return 'unknown';

		const diffSec = Math.max(0, Math.floor((Date.now() - then) / 1000));
		const hours = Math.floor(diffSec / 3600);
		const minutes = Math.floor((diffSec % 3600) / 60);
		const seconds = diffSec % 60;

		if (hours > 0) {
			if (minutes > 0) return `${hours}h ${minutes}m`;
			return `${hours}h`;
		}

		if (minutes > 0) {
			if (seconds > 0) return `${minutes}m ${seconds}s`;
			return `${minutes}m`;
		}

		return `${seconds}s`;
	}

	const checkedDisplay = $derived.by(() => {
		if (!service.lastChecked) return 'not yet checked';
		return `checked ${formatRelativeTime(service.lastChecked)}`;
	});

	const stateDisplay = $derived.by(() => {
		if (service.status === 'unhealthy') {
			return `unhealthy for ${formatUnhealthyDuration(service.lastStateChange)}`;
		}

		const label = statusLabelMap[service.status];
		const time = formatRelativeTime(service.lastStateChange);
		return `${label} ${time}`;
	});

	const stateColor = $derived.by(() => healthColorMap[service.status]);

	const errorLine = $derived.by(() => {
		if (service.status !== 'unhealthy' || !service.errorSnippet) return null;
		const snippet = service.errorSnippet;
		return snippet.length > 80 ? snippet.slice(0, 80) + '…' : snippet;
	});

	const positionClasses = $derived.by(() => {
		return position === 'above' ? 'bottom-full mb-1' : 'top-full mt-1';
	});
</script>

{#if visible}
	<div
		{id}
		role="tooltip"
		class="absolute left-0 z-50 bg-surface-1 border border-overlay-1 rounded-sm p-2 text-[11px] text-subtext-0 max-w-[400px] {positionClasses}"
	>
		<div>{checkedDisplay}</div>
		<div class={stateColor}>{stateDisplay}</div>
		{#if errorLine}
			<div class="truncate">{errorLine}</div>
		{/if}
	</div>
{/if}
