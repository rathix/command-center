<script lang="ts">
	import { getGroupedServices } from '$lib/serviceStore.svelte';
	import SectionLabel from './SectionLabel.svelte';
	import ServiceRow from './ServiceRow.svelte';

	const groups = $derived.by(() => getGroupedServices());
	const showLabels = $derived.by(
		() => groups.needsAttention.length > 0 && groups.healthy.length > 0
	);
	const hasServices = $derived.by(
		() => groups.needsAttention.length + groups.healthy.length > 0
	);
</script>

{#if hasServices}
	<ul class="m-0 list-none p-0">
		{#if showLabels}
			<SectionLabel label="needs attention" />
			{#each groups.needsAttention as service, i (`${service.namespace}/${service.name}`)}
				<ServiceRow {service} odd={i % 2 !== 0} />
			{/each}
			<SectionLabel label="healthy" />
			{#each groups.healthy as service, i (`${service.namespace}/${service.name}`)}
				<ServiceRow {service} odd={(groups.needsAttention.length + i) % 2 !== 0} />
			{/each}
		{:else}
			{@const allServices = groups.needsAttention.length > 0 ? groups.needsAttention : groups.healthy}
			{#each allServices as service, i (`${service.namespace}/${service.name}`)}
				<ServiceRow {service} odd={i % 2 !== 0} />
			{/each}
		{/if}
	</ul>
{/if}
