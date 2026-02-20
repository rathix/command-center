import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, beforeEach } from 'vitest';
import ServiceList from './ServiceList.svelte';
import { replaceAll, _resetForTesting } from '$lib/serviceStore.svelte';
import type { Service } from '$lib/types';

function makeService(overrides: Partial<Service> & { name: string }): Service {
	return {
		displayName: overrides.displayName ?? overrides.name,
		namespace: 'default',
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

describe('ServiceList', () => {
	it('has role="list" when services are present', () => {
		replaceAll([makeService({ name: 'alpha' })]);
		render(ServiceList);
		expect(screen.getByRole('list')).toBeInTheDocument();
	});

	it('renders correct number of service rows', () => {
		replaceAll([
			makeService({ name: 'alpha' }),
			makeService({ name: 'bravo' }),
			makeService({ name: 'charlie' })
		]);
		render(ServiceList);
		const items = screen.getAllByRole('listitem');
		expect(items).toHaveLength(3);
	});

	it('renders services in alphabetical order when all unknown', () => {
		replaceAll([
			makeService({ name: 'charlie' }),
			makeService({ name: 'alpha' }),
			makeService({ name: 'bravo' })
		]);
		render(ServiceList);
		const items = screen.getAllByRole('listitem');
		// All unknown = all in needsAttention with no labels, flat alphabetical
		expect(items[0]).toHaveTextContent('alpha');
		expect(items[1]).toHaveTextContent('bravo');
		expect(items[2]).toHaveTextContent('charlie');
	});

	it('passes odd prop correctly for alternating backgrounds', () => {
		replaceAll([
			makeService({ name: 'alpha' }),
			makeService({ name: 'bravo' }),
			makeService({ name: 'charlie' })
		]);
		render(ServiceList);
		const items = screen.getAllByRole('listitem');
		// index 0 → odd=false (no bg-surface-0), index 1 → odd=true, index 2 → odd=false
		expect(items[0]).not.toHaveClass('bg-surface-0');
		expect(items[1]).toHaveClass('bg-surface-0');
		expect(items[2]).not.toHaveClass('bg-surface-0');
	});

	it('renders empty when store has no services', () => {
		render(ServiceList);
		expect(screen.queryByRole('list')).not.toBeInTheDocument();
		expect(screen.queryAllByRole('listitem')).toHaveLength(0);
	});

	describe('grouped rendering with section labels', () => {
		it('renders section labels when mixed health states exist', () => {
			replaceAll([
				makeService({ name: 'bad-svc', status: 'unhealthy' }),
				makeService({ name: 'good-svc', status: 'healthy' })
			]);
			render(ServiceList);
			const headings = screen.getAllByRole('heading');
			expect(headings).toHaveLength(2);
			expect(headings[0]).toHaveTextContent('needs attention');
			expect(headings[1]).toHaveTextContent('healthy');
		});

		it('renders unhealthy/authBlocked services under "needs attention" and healthy under "healthy"', () => {
			replaceAll([
				makeService({ name: 'good-a', status: 'healthy' }),
				makeService({ name: 'good-b', status: 'healthy' }),
				makeService({ name: 'bad-a', status: 'unhealthy' }),
				makeService({ name: 'blocked-a', status: 'authBlocked' })
			]);
			render(ServiceList);
			// Section labels have role="heading", service rows have role="listitem"
			const headings = screen.getAllByRole('heading');
			const items = screen.getAllByRole('listitem');
			expect(headings).toHaveLength(2);
			expect(items).toHaveLength(4);
			// Headings: "needs attention" then "healthy"
			expect(headings[0]).toHaveTextContent('needs attention');
			expect(headings[1]).toHaveTextContent('healthy');
			// Service rows: sorted by status priority, then alphabetically
			expect(items[0]).toHaveTextContent('bad-a');
			expect(items[1]).toHaveTextContent('blocked-a');
			expect(items[2]).toHaveTextContent('good-a');
			expect(items[3]).toHaveTextContent('good-b');
		});

		it('renders NO section labels when all services are healthy', () => {
			replaceAll([
				makeService({ name: 'alpha', status: 'healthy' }),
				makeService({ name: 'bravo', status: 'healthy' })
			]);
			render(ServiceList);
			expect(screen.queryByRole('heading')).not.toBeInTheDocument();
			const items = screen.getAllByRole('listitem');
			expect(items).toHaveLength(2);
			expect(items[0]).toHaveTextContent('alpha');
			expect(items[1]).toHaveTextContent('bravo');
		});

		it('renders NO section labels when all services are unhealthy', () => {
			replaceAll([
				makeService({ name: 'bad-a', status: 'unhealthy' }),
				makeService({ name: 'bad-b', status: 'authBlocked' })
			]);
			render(ServiceList);
			expect(screen.queryByRole('heading')).not.toBeInTheDocument();
			const items = screen.getAllByRole('listitem');
			expect(items).toHaveLength(2);
		});

		it('sorts alphabetically within each section group', () => {
			replaceAll([
				makeService({ name: 'zebra', status: 'healthy' }),
				makeService({ name: 'alpha', status: 'healthy' }),
				makeService({ name: 'delta', status: 'unhealthy' }),
				makeService({ name: 'bravo', status: 'unhealthy' })
			]);
			render(ServiceList);
			const items = screen.getAllByRole('listitem');
			// needsAttention: bravo, delta; healthy: alpha, zebra
			expect(items[0]).toHaveTextContent('bravo');
			expect(items[1]).toHaveTextContent('delta');
			expect(items[2]).toHaveTextContent('alpha');
			expect(items[3]).toHaveTextContent('zebra');
		});

		it('continues alternating bg-surface-0 across sections without reset', () => {
			replaceAll([
				makeService({ name: 'bad-a', status: 'unhealthy' }),
				makeService({ name: 'bad-b', status: 'authBlocked' }),
				makeService({ name: 'good-a', status: 'healthy' })
			]);
			render(ServiceList);
			const items = screen.getAllByRole('listitem');
			// items[0] = bad-a (row index 0 → odd=false → no bg-surface-0)
			// items[1] = bad-b (row index 1 → odd=true → bg-surface-0)
			// items[2] = good-a (row index 2 → offset=2+0=2, odd=false → no bg-surface-0)
			expect(items[0]).not.toHaveClass('bg-surface-0');
			expect(items[1]).toHaveClass('bg-surface-0');
			expect(items[2]).not.toHaveClass('bg-surface-0');
		});

		it('section labels are not clickable — they are li elements with heading role', () => {
			replaceAll([
				makeService({ name: 'bad-svc', status: 'unhealthy' }),
				makeService({ name: 'good-svc', status: 'healthy' })
			]);
			render(ServiceList);
			const headings = screen.getAllByRole('heading');
			for (const heading of headings) {
				expect(heading.tagName.toLowerCase()).toBe('li');
				expect(heading.querySelector('a')).toBeNull();
			}
		});
	});
});
