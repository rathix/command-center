<script lang="ts">
	interface Props {
		data: number[];
		width?: number;
		height?: number;
		color?: string;
	}

	const { data, width = 80, height = 24, color = 'currentColor' }: Props = $props();

	const points = $derived.by(() => {
		if (data.length === 0) return '';
		if (data.length === 1) return '';

		const xStep = width / Math.max(data.length - 1, 1);
		return data
			.map((value, i) => {
				const x = i * xStep;
				const y = height - (Math.min(Math.max(value, 0), 100) / 100) * height;
				return `${x.toFixed(1)},${y.toFixed(1)}`;
			})
			.join(' ');
	});

	const areaPoints = $derived.by(() => {
		if (data.length < 2) return '';
		const xStep = width / Math.max(data.length - 1, 1);
		const linePoints = data.map((value, i) => {
			const x = i * xStep;
			const y = height - (Math.min(Math.max(value, 0), 100) / 100) * height;
			return `${x.toFixed(1)},${y.toFixed(1)}`;
		});
		return `0,${height} ${linePoints.join(' ')} ${width},${height}`;
	});

	const singlePoint = $derived.by(() => {
		if (data.length !== 1) return null;
		const x = width / 2;
		const y = height - (Math.min(Math.max(data[0], 0), 100) / 100) * height;
		return { x, y };
	});
</script>

{#if data.length === 0}
	<!-- No data — render nothing -->
{:else if data.length === 1}
	<svg {width} {height} viewBox="0 0 {width} {height}" role="img" aria-label="sparkline">
		<circle cx={singlePoint!.x} cy={singlePoint!.y} r="2" fill={color} />
	</svg>
{:else}
	<svg {width} {height} viewBox="0 0 {width} {height}" role="img" aria-label="sparkline">
		<polygon points={areaPoints} fill={color} opacity="0.15" />
		<polyline points={points} fill="none" stroke={color} stroke-width="1.5" />
	</svg>
{/if}
