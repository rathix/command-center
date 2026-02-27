import { render, screen } from '@testing-library/svelte';
import { tick } from 'svelte';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import HoverTooltip from './HoverTooltip.svelte';
import type { Service } from '$lib/types';

function makeService(overrides: Partial<Service> = {}): Service {
	return {
		name: 'grafana',
		displayName: 'grafana',
		namespace: 'monitoring',
		group: 'monitoring',
		url: 'https://grafana.example.com',
		status: 'healthy',
		compositeStatus: overrides.compositeStatus ?? overrides.status ?? 'healthy',
		readyEndpoints: null,
		totalEndpoints: null,
		authGuarded: false,
		httpCode: 200,
		responseTimeMs: 142,
		lastChecked: '2026-02-20T10:00:00Z',
		lastStateChange: '2026-02-20T08:00:00Z',
		errorSnippet: null,
		podDiagnostic: null,
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

	it('renders state duration with health-colored text for healthy service', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'healthy',
					lastStateChange: '2026-02-20T08:00:00Z'
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		const durationEl = screen.getByText(/healthy for 2h/);
		expect(durationEl).toBeInTheDocument();
		expect(durationEl).not.toHaveTextContent(/ago/);
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
				left: 0,
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
				left: 0,
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
				left: 0,
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
				left: 0,
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
				left: 0,
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
				left: 0,
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

	it('state duration ticks while tooltip is visible', async () => {
		vi.setSystemTime(new Date('2026-02-20T10:00:00Z'));
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'unhealthy',
					lastStateChange: '2026-02-20T09:55:28Z'
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText(/unhealthy for 4m 32s/)).toBeInTheDocument();
		vi.advanceTimersByTime(5000);
		await tick();
		expect(screen.getByText(/unhealthy for 4m 37s/)).toBeInTheDocument();
	});

	it('renders source line for kubernetes service', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ source: 'kubernetes', namespace: 'media' }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText('Source: Kubernetes / media')).toBeInTheDocument();
	});

	it('renders source line for config service', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ source: 'config' }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText('Source: Custom config')).toBeInTheDocument();
	});

	it('does not render source line when source is undefined', () => {
		render(HoverTooltip, {
			props: {
				service: makeService(),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.queryByText(/Source:/)).not.toBeInTheDocument();
	});

	it('shows correct duration from lastStateChange when lastChecked is null (pre-populated startup state)', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'healthy',
					lastChecked: null,
					lastStateChange: '2026-02-20T08:00:00Z'
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		const durationEl = screen.getByText(/healthy for 2h/);
		expect(durationEl).toBeInTheDocument();
		expect(durationEl).toHaveClass('text-health-ok');
	});

	it('shows day-scale duration for lastStateChange days ago with null lastChecked', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'healthy',
					lastChecked: null,
					lastStateChange: '2026-02-18T10:00:00Z'
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText(/healthy for 2d/)).toBeInTheDocument();
	});

	it('renders with graceful defaults when lastStateChange is null', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ status: 'unknown', lastChecked: null, lastStateChange: null }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText(/unknown for unknown/)).toBeInTheDocument();
		expect(screen.getByRole('tooltip')).toBeInTheDocument();
	});

	it('shows pod readiness line for K8s service', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ readyEndpoints: 3, totalEndpoints: 3 }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText('Pods: 3/3 ready')).toBeInTheDocument();
	});

	it('shows partial pod readiness', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ readyEndpoints: 1, totalEndpoints: 3 }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText('Pods: 1/3 ready')).toBeInTheDocument();
	});

	it('does not show pod readiness for custom service (null endpoints)', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ readyEndpoints: null, totalEndpoints: null }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.queryByText(/Pods:/)).not.toBeInTheDocument();
	});

	it('shows auth-guarded line when authGuarded is true', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ authGuarded: true }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText('Auth-guarded (forward auth)')).toBeInTheDocument();
	});

	it('does not show auth-guarded line when authGuarded is false', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ authGuarded: false }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.queryByText(/Auth-guarded/)).not.toBeInTheDocument();
	});

	it('shows degraded hint for degraded service', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ status: 'degraded', compositeStatus: 'degraded' }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText(/Pods are ready but HTTP probe failed/)).toBeInTheDocument();
	});

	it('degraded hint has text-health-degraded class', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ status: 'degraded', compositeStatus: 'degraded' }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText(/Pods are ready but HTTP probe failed/)).toHaveClass('text-health-degraded');
	});

	it('does not show degraded hint for healthy service', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ status: 'healthy', compositeStatus: 'healthy' }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.queryByText(/Pods are ready/)).not.toBeInTheDocument();
	});

	it('degraded service shows text-health-degraded color on state duration line', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ status: 'degraded', compositeStatus: 'degraded' }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		const durationEl = screen.getByText(/degraded for/);
		expect(durationEl).toHaveClass('text-health-degraded');
	});

	it('uses compositeStatus for degraded state display when raw status differs', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({ status: 'healthy', compositeStatus: 'degraded' }),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText(/degraded for/)).toBeInTheDocument();
		expect(screen.getByText(/Pods are ready but HTTP probe failed/)).toBeInTheDocument();
	});

	it('suppresses unhealthy error snippet when compositeStatus is healthy', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'unhealthy',
					compositeStatus: 'healthy',
					errorSnippet: 'stale unhealthy message'
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.queryByText(/stale unhealthy message/)).not.toBeInTheDocument();
	});
});

describe('HoverTooltip pod diagnostics', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-02-20T10:00:12Z'));
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('shows diagnostic reason for unhealthy K8s service', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'unhealthy',
					source: 'kubernetes',
					podDiagnostic: { reason: 'CrashLoopBackOff', restartCount: 5 }
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText('CrashLoopBackOff · 5 restarts')).toBeInTheDocument();
	});

	it('shows restart count only when no reason', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'unhealthy',
					source: 'kubernetes',
					podDiagnostic: { reason: null, restartCount: 3 }
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText('3 restarts')).toBeInTheDocument();
	});

	it('shows singular restart for count of 1', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'unhealthy',
					source: 'kubernetes',
					podDiagnostic: { reason: null, restartCount: 1 }
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.getByText('1 restart')).toBeInTheDocument();
	});

	it('shows no pod diag line when podDiagnostic is null', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'unhealthy',
					source: 'kubernetes',
					podDiagnostic: null
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.queryByText(/restart/)).not.toBeInTheDocument();
		expect(screen.queryByText(/CrashLoopBackOff/)).not.toBeInTheDocument();
	});

	it('shows no pod diag line for config source services', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'unhealthy',
					source: 'config',
					podDiagnostic: null
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.queryByText(/restart/)).not.toBeInTheDocument();
	});

	it('shows no pod diag line when reason is null and restartCount is 0', () => {
		render(HoverTooltip, {
			props: {
				service: makeService({
					status: 'unhealthy',
					source: 'kubernetes',
					podDiagnostic: { reason: null, restartCount: 0 }
				}),
				visible: true,
				position: 'below',
				left: 0,
				id: 'tooltip-test'
			}
		});
		expect(screen.queryByText(/restart/)).not.toBeInTheDocument();
	});
});
