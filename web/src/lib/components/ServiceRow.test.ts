import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ServiceRow from './ServiceRow.svelte';
import type { Service } from '$lib/types';

function makeService(overrides: Partial<Service> = {}): Service {
	return {
		name: 'grafana',
		displayName: 'grafana',
		namespace: 'monitoring',
		url: 'https://grafana.example.com',
		status: 'unknown',
		httpCode: null,
		responseTimeMs: null,
		lastChecked: null,
		lastStateChange: null,
		errorSnippet: null,
		...overrides
	};
}

describe('ServiceRow', () => {
	it('renders service display name text', () => {
		render(ServiceRow, {
			props: { service: makeService({ displayName: 'my-service' }), odd: false }
		});
		expect(screen.getByText('my-service')).toBeInTheDocument();
	});

	it('renders service URL text', () => {
		render(ServiceRow, {
			props: { service: makeService({ url: 'https://svc.example.com' }), odd: false }
		});
		expect(screen.getByText('https://svc.example.com')).toBeInTheDocument();
	});

	it('renders as <a> with correct href', () => {
		render(ServiceRow, {
			props: { service: makeService({ url: 'https://grafana.example.com' }), odd: false }
		});
		const link = screen.getByRole('link');
		expect(link.tagName).toBe('A');
		expect(link).toHaveAttribute('href', 'https://grafana.example.com/');
	});

	it('has target="_blank" and rel="noopener noreferrer"', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const link = screen.getByRole('link');
		expect(link).toHaveAttribute('target', '_blank');
		expect(link).toHaveAttribute('rel', 'noopener noreferrer');
	});

	it('renders TuiDot with service status', () => {
		render(ServiceRow, {
			props: { service: makeService({ status: 'unknown' }), odd: false }
		});
		const dot = screen.getByRole('img');
		expect(dot).toHaveAttribute('aria-label', 'unknown');
	});

	it('has bg-surface-0 class when odd=true', () => {
		render(ServiceRow, { props: { service: makeService(), odd: true } });
		const listItem = screen.getByRole('listitem');
		expect(listItem).toHaveClass('bg-surface-0');
	});

	it('does not have bg-surface-0 class when odd=false', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const listItem = screen.getByRole('listitem');
		expect(listItem).not.toHaveClass('bg-surface-0');
	});

	it('has role="listitem"', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		expect(screen.getByRole('listitem')).toBeInTheDocument();
	});

	it('has hover transition classes', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const listItem = screen.getByRole('listitem');
		expect(listItem).toHaveClass('transition-colors');
		expect(listItem).toHaveClass('duration-150');
	});

	it('has cursor-pointer class', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const link = screen.getByRole('link');
		expect(link).toHaveClass('cursor-pointer');
	});

	it('has fixed 46px height', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const listItem = screen.getByRole('listitem');
		expect(listItem).toHaveClass('h-[46px]');
	});

	it('sanitizes unsafe urls to a safe fallback link', () => {
		render(ServiceRow, {
			props: { service: makeService({ url: 'javascript:alert(1)' }), odd: false }
		});
		const link = screen.getByRole('link');
		expect(link).toHaveAttribute('href', '#');
		expect(link).toHaveAttribute('aria-disabled', 'true');
	});
});
