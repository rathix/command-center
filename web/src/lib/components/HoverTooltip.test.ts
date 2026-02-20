import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import HoverTooltip from './HoverTooltip.svelte';
import type { Service } from '$lib/types';

function makeService(overrides: Partial<Service> = {}): Service {
	return {
		name: 'grafana',
		displayName: 'grafana',
		namespace: 'monitoring',
		url: 'https://grafana.example.com',
		status: 'healthy',
		httpCode: 200,
		responseTimeMs: 142,
		lastChecked: '2026-02-20T10:00:00Z',
		lastStateChange: '2026-02-20T08:00:00Z',
		errorSnippet: null,
		...overrides
	};
}

describe('HoverTooltip', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-02-20T10:00:12Z'));
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('renders "checked {time} ago" from lastChecked', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ lastChecked: '2026-02-20T10:00:00Z' }),
				visible: true,
				position: 'below',
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText(/checked 12s ago/)).toBeInTheDocument();
	});

	it('renders "not yet checked" when lastChecked is null', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ lastChecked: null }),
				visible: true,
				position: 'below',
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText(/not yet checked/)).toBeInTheDocument();
	});

	it('renders state duration with health-colored text for healthy service', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'healthy',
					lastStateChange: '2026-02-20T08:00:00Z'
				}),
				visible: true,
				position: 'below',
				id: 'tooltip-test'
			}
		});
		const durationEl = screen.getByText(/healthy since 2h ago/);
		expect(durationEl).toBeInTheDocument();
		expect(durationEl).toHaveClass('text-health-ok');
	});

	it('renders state duration with error color for unhealthy service', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'unhealthy',
					lastStateChange: '2026-02-20T09:55:40Z'
				}),
				visible: true,
				position: 'below',
				id: 'tooltip-test'
			}
		});
		const durationEl = screen.getByText(/unhealthy for 4m 32s/);
		expect(durationEl).toBeInTheDocument();
		expect(durationEl).toHaveClass('text-health-error');
	});

	it('renders error snippet only when status is unhealthy and errorSnippet is not null', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'unhealthy',
					errorSnippet: 'Connection refused'
				}),
				visible: true,
				position: 'below',
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText(/Connection refused/)).toBeInTheDocument();
	});

	it('does NOT render error snippet when status is healthy even if errorSnippet is set', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'healthy',
					errorSnippet: 'Some old error'
				}),
				visible: true,
				position: 'below',
				id: 'tooltip-test'
			}
		});
		expect(screen.queryByText(/Some old error/)).not.toBeInTheDocument();
	});

	it('has role="tooltip" attribute', () => {
		render(HoverTooltip, {
			props: {
				service: makeService(),
				visible: true,
				position: 'below',
				id: 'tooltip-test'
			}
		});
		expect(screen.getByRole('tooltip')).toBeInTheDocument();
	});

	it('has correct id prop for aria-describedby linkage', () => {
		render(HoverTooltip, {
			props: {
				service: makeService(),
				visible: true,
				position: 'below',
				id: 'tooltip-monitoring-grafana'
			}
		});
		expect(screen.getByRole('tooltip')).toHaveAttribute('id', 'tooltip-monitoring-grafana');
	});

	it('has correct styling classes', () => {
		render(HoverTooltip, {
			props: {
				service: makeService(),
				visible: true,
				position: 'below',
				id: 'tooltip-test'
			}
		});
		const tooltip = screen.getByRole('tooltip');
		expect(tooltip).toHaveClass('bg-surface-1');
		expect(tooltip).toHaveClass('border-overlay-1');
		expect(tooltip).toHaveClass('rounded-sm');
		expect(tooltip).toHaveClass('p-2');
		expect(tooltip).toHaveClass('text-[11px]');
		expect(tooltip).toHaveClass('max-w-[400px]');
	});

	it('renders with graceful defaults when lastChecked and lastStateChange are null', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ lastChecked: null, lastStateChange: null }),
				visible: true,
				position: 'below',
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText(/not yet checked/)).toBeInTheDocument();
		expect(screen.getByRole('tooltip')).toBeInTheDocument();
	});
});
