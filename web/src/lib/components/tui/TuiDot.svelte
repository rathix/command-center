<script lang="ts">
	import type { HealthStatus } from '$lib/types';

	let { status }: { status: HealthStatus } = $props();

	const colorMap: Record<HealthStatus, string> = {
		healthy: 'bg-health-ok',
		degraded: 'bg-health-degraded',
		unhealthy: 'bg-health-error',
		unknown: 'bg-health-unknown'
	};

	const labelMap: Record<HealthStatus, string> = {
		healthy: 'healthy',
		degraded: 'degraded',
		unhealthy: 'unhealthy',
		unknown: 'unknown'
	};
</script>

<span
	class="inline-block h-2 w-2 shrink-0 rounded-full transition-colors duration-300 {colorMap[status]}"
	role="img"
	aria-label={labelMap[status]}
></span>
