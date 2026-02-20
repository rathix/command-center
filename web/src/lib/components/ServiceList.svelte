<script lang="ts">
	import { getSortedServices } from '$lib/serviceStore.svelte';
	import ServiceRow from './ServiceRow.svelte';
</script>

{#if getSortedServices().length > 0}
	<ul class="m-0 list-none p-0">
		{#each getSortedServices() as service, index (`${service.namespace}/${service.name}`)}
			<ServiceRow {service} odd={index % 2 !== 0} />
		{/each}
	</ul>
{/if}
