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
});
