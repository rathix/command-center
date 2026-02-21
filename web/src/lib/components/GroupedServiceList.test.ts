import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, beforeEach } from 'vitest';
import { tick } from 'svelte';
import GroupedServiceList from './GroupedServiceList.svelte';
import { replaceAll, addOrUpdate, _resetForTesting } from '$lib/serviceStore.svelte';
import type { Service } from '$lib/types';

function makeService(overrides: Partial<Service> & { name: string }): Service {
	return {
		displayName: overrides.displayName ?? overrides.name,
		namespace: 'default',
		group: 'default',
		url: 'https://test.example.com',
		status: 'unknown',
		httpCode: null,
		responseTimeMs: null,
		lastChecked: null,
		lastStateChange: null,
		errorSnippet: null,
		...overrides
	};
}

beforeEach(() => {
	_resetForTesting();
});

describe('GroupedServiceList', () => {
	it('renders GroupHeader for each service group', () => {
		replaceAll(
			[
				makeService({ name: 'svc-a', group: 'infra', status: 'unhealthy' }),
				makeService({ name: 'svc-b', group: 'media', status: 'unhealthy' })
			],
			'v1.0.0'
		);
		render(GroupedServiceList);
		const buttons = screen.getAllByRole('button');
		expect(buttons).toHaveLength(2);
		expect(buttons[0]).toHaveTextContent('infra');
		expect(buttons[1]).toHaveTextContent('media');
	});

	it('renders ServiceRow entries under each expanded group', () => {
		replaceAll(
			[
				makeService({ name: 'svc-a', group: 'infra', status: 'unhealthy' }),
				makeService({ name: 'svc-b', group: 'infra', status: 'unhealthy' }),
				makeService({ name: 'svc-c', group: 'media', status: 'unhealthy' })
			],
			'v1.0.0'
		);
		render(GroupedServiceList);
		const items = screen.getAllByRole('listitem');
		expect(items).toHaveLength(3);
	});

	it('groups with unhealthy services are auto-expanded (services visible)', () => {
		replaceAll(
			[makeService({ name: 'svc-a', group: 'infra', status: 'unhealthy' })],
			'v1.0.0'
		);
		render(GroupedServiceList);
		const button = screen.getByRole('button');
		expect(button).toHaveAttribute('aria-expanded', 'true');
		expect(screen.getAllByRole('listitem')).toHaveLength(1);
	});

	it('groups with all healthy services are collapsed (services hidden)', () => {
		replaceAll(
			[makeService({ name: 'svc-a', group: 'infra', status: 'healthy' })],
			'v1.0.0'
		);
		render(GroupedServiceList);
		const button = screen.getByRole('button');
		expect(button).toHaveAttribute('aria-expanded', 'false');
		expect(screen.queryAllByRole('listitem')).toHaveLength(0);
	});

	it('collapsed groups do not render ServiceRow entries', () => {
		replaceAll(
			[
				makeService({ name: 'svc-a', group: 'healthy-group', status: 'healthy' }),
				makeService({ name: 'svc-b', group: 'healthy-group', status: 'healthy' })
			],
			'v1.0.0'
		);
		render(GroupedServiceList);
		expect(screen.queryAllByRole('listitem')).toHaveLength(0);
	});

	it('each group service list has an id matching the GroupHeader aria-controls', () => {
		replaceAll(
			[makeService({ name: 'svc-a', group: 'infra', status: 'unhealthy' })],
			'v1.0.0'
		);
		render(GroupedServiceList);
		const button = screen.getByRole('button');
		const controlsId = button.getAttribute('aria-controls');
		expect(controlsId).toBe('group-infra-services');
		const list = document.getElementById(controlsId!);
		expect(list).toBeTruthy();
		expect(list!.tagName.toLowerCase()).toBe('ul');
	});

	it('collapsed groups still render a controlled list element for aria-controls', () => {
		replaceAll(
			[makeService({ name: 'svc-a', group: 'infra', status: 'healthy' })],
			'v1.0.0'
		);
		render(GroupedServiceList);
		const button = screen.getByRole('button');
		expect(button).toHaveAttribute('aria-expanded', 'false');
		const controlsId = button.getAttribute('aria-controls');
		const list = document.getElementById(controlsId!);
		expect(list).toBeTruthy();
		expect(list).toHaveAttribute('hidden');
	});

	it('sanitizes group names when generating aria-controls ids', () => {
		replaceAll(
			[makeService({ name: 'svc-a', group: 'infra / ops', status: 'unhealthy' })],
			'v1.0.0'
		);
		render(GroupedServiceList);
		const button = screen.getByRole('button');
		const controlsId = button.getAttribute('aria-controls');
		expect(controlsId).toBe('group-infra%20%2F%20ops-services');
		expect(document.getElementById(controlsId!)).toBeTruthy();
	});

	it('manual collapse of problem groups persists for the session across store updates', async () => {
		replaceAll(
			[makeService({ name: 'svc-a', group: 'infra', status: 'unhealthy' })],
			'v1.0.0'
		);
		render(GroupedServiceList);
		const button = screen.getByRole('button');
		expect(button).toHaveAttribute('aria-expanded', 'true');
		expect(screen.getAllByRole('listitem')).toHaveLength(1);

		await fireEvent.click(button);
		await tick();
		expect(button).toHaveAttribute('aria-expanded', 'false');
		expect(screen.queryAllByRole('listitem')).toHaveLength(0);

		addOrUpdate(makeService({ name: 'svc-b', group: 'infra', status: 'unhealthy' }));
		await tick();
		expect(button).toHaveAttribute('aria-expanded', 'false');
		expect(screen.queryAllByRole('listitem')).toHaveLength(0);
	});

	it('alternating odd backgrounds reset per group', () => {
		replaceAll(
			[
				makeService({ name: 'svc-a', group: 'alpha', status: 'unhealthy' }),
				makeService({ name: 'svc-b', group: 'alpha', status: 'unhealthy' }),
				makeService({ name: 'svc-c', group: 'alpha', status: 'unhealthy' }),
				makeService({ name: 'svc-d', group: 'beta', status: 'unhealthy' }),
				makeService({ name: 'svc-e', group: 'beta', status: 'unhealthy' })
			],
			'v1.0.0'
		);
		render(GroupedServiceList);
		const items = screen.getAllByRole('listitem');
		// alpha group: index 0 (no bg), 1 (bg), 2 (no bg)
		expect(items[0]).not.toHaveClass('bg-surface-0');
		expect(items[1]).toHaveClass('bg-surface-0');
		expect(items[2]).not.toHaveClass('bg-surface-0');
		// beta group: index resets — 0 (no bg), 1 (bg)
		expect(items[3]).not.toHaveClass('bg-surface-0');
		expect(items[4]).toHaveClass('bg-surface-0');
	});

	it('no SectionLabel elements are rendered (no "needs attention" / "healthy" headings)', () => {
		replaceAll(
			[
				makeService({ name: 'svc-a', group: 'infra', status: 'unhealthy' }),
				makeService({ name: 'svc-b', group: 'infra', status: 'healthy' })
			],
			'v1.0.0'
		);
		render(GroupedServiceList);
		expect(screen.queryByRole('heading')).not.toBeInTheDocument();
		expect(screen.queryByText('needs attention')).not.toBeInTheDocument();
		expect(screen.queryByText('healthy')).not.toBeInTheDocument();
	});

	it('empty store renders nothing', () => {
		render(GroupedServiceList);
		expect(screen.queryByRole('button')).not.toBeInTheDocument();
		expect(screen.queryByRole('list')).not.toBeInTheDocument();
		expect(screen.queryAllByRole('listitem')).toHaveLength(0);
	});

	it('single group renders correctly (GroupHeader + services)', () => {
		replaceAll(
			[
				makeService({ name: 'svc-a', group: 'only-group', status: 'unhealthy' }),
				makeService({ name: 'svc-b', group: 'only-group', status: 'healthy' })
			],
			'v1.0.0'
		);
		render(GroupedServiceList);
		const buttons = screen.getAllByRole('button');
		expect(buttons).toHaveLength(1);
		expect(buttons[0]).toHaveTextContent('only-group');
		// Group has unhealthy service, so expanded
		expect(buttons[0]).toHaveAttribute('aria-expanded', 'true');
		const items = screen.getAllByRole('listitem');
		expect(items).toHaveLength(2);
	});
});
