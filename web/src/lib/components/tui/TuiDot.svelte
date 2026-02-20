<script lang="ts">
	import type { HealthStatus } from '$lib/types';

	let { status }: { status: HealthStatus } = $props();

	const colorMap: Record<HealthStatus, string> = {
		healthy: 'bg-health-ok',
		unhealthy: 'bg-health-error',
		authBlocked: 'bg-health-auth-blocked',
		unknown: 'bg-health-unknown'
	};

	const labelMap: Record<HealthStatus, string> = {
		healthy: 'healthy',
		unhealthy: 'unhealthy',
		authBlocked: 'authentication blocked',
		unknown: 'unknown'
	};
</script>

<span
	class="inline-block h-2 w-2 shrink-0 rounded-full transition-colors duration-300 {colorMap[status]}"
	role="img"
	aria-label={labelMap[status]}
></span>
