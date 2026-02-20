<script lang="ts">
	import { getSortedServices } from '$lib/serviceStore.svelte';
	import ServiceRow from './ServiceRow.svelte';
</script>

<div role="list">
	{#each getSortedServices() as service, index (`${service.namespace}/${service.name}`)}
		<ServiceRow {service} odd={index % 2 !== 0} />
	{/each}
</div>
