<script lang="ts">
	import { getServiceGroups } from '$lib/serviceStore.svelte';
	import GroupHeader from './GroupHeader.svelte';
	import ServiceRow from './ServiceRow.svelte';

	const groups = $derived.by(() => getServiceGroups());

	function getControlsId(groupName: string): string {
		const sanitized = groupName
			.trim()
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '');
		return sanitized ? `group-${sanitized}-services` : 'group-ungrouped-services';
	}
</script>

{#if groups.length > 0}
	<div>
		{#each groups as group (group.name)}
			{@const controlsId = getControlsId(group.name)}
			<GroupHeader {group} {controlsId} />
			<ul id={controlsId} class="m-0 list-none p-0" hidden={!group.expanded}>
				{#if group.expanded}
					{#each group.services as service, i (`${service.namespace}/${service.name}`)}
						<ServiceRow {service} odd={i % 2 !== 0} />
					{/each}
				{/if}
			</ul>
		{/each}
	</div>
{/if}
