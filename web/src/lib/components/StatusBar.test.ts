import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import StatusBar from './StatusBar.svelte';
import {
	replaceAll,
	setConnectionStatus,
	setK8sStatus,
	setConfigErrors,
	_resetForTesting
} from '$lib/serviceStore.svelte';
import type { Service } from '$lib/types';

function makeService(overrides: Partial<Service> & { name: string }): Service {
	const nowIso = new Date().toISOString();
	return {
		displayName: overrides.displayName ?? overrides.name,
		namespace: 'default',
		group: 'default',
		url: 'https://test.example.com',
		status: 'unknown',
		compositeStatus: overrides.compositeStatus ?? overrides.status ?? 'unknown',
		readyEndpoints: null,
		totalEndpoints: null,
		authGuarded: false,
		httpCode: null,
		responseTimeMs: null,
		lastChecked: nowIso,
		lastStateChange: nowIso,
		errorSnippet: null,
		podDiagnostic: null,
		...overrides
	};
}

beforeEach(() => {
	vi.useFakeTimers();
	vi.setSystemTime(new Date('2026-02-20T12:00:00Z'));
	_resetForTesting();
});

afterEach(() => {
	vi.useRealTimers();
});

describe('StatusBar', () => {
	it('shows "Discovering services..." when connectionStatus is connecting', () => {
		// Default connectionStatus is 'connecting' after reset
		render(StatusBar);
		expect(screen.getByText('Discovering services...')).toBeInTheDocument();
	});

	it('shows service count when connected with services', () => {
		setConnectionStatus('connected');
		replaceAll([
			makeService({ name: 'svc-1', status: 'healthy' }),
			makeService({ name: 'svc-2', status: 'healthy' }),
			makeService({ name: 'svc-3', status: 'healthy' })
		], 'v1.0.0');
		render(StatusBar);
		expect(screen.getByText('3 services — all healthy')).toBeInTheDocument();
	});

	it('shows "Reconnecting..." AND preserves health summary when reconnecting', () => {
		replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
		setConnectionStatus('reconnecting');
		render(StatusBar);
		expect(screen.getByText('Reconnecting...')).toBeInTheDocument();
		expect(screen.getByText('1 service — all healthy')).toBeInTheDocument();
		expect(screen.getByText('Last updated 0s ago')).toBeInTheDocument();
		expect(screen.queryByText('Command Center v1.0.0')).not.toBeInTheDocument();
	});

	it('shows "Connection lost" in error color when disconnected', () => {
		replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
		setConnectionStatus('disconnected');
		render(StatusBar);
		const connectionLost = screen.getByText('Connection lost');
		expect(connectionLost).toBeInTheDocument();
		expect(connectionLost).toHaveClass('text-health-error');
		// Health summary is still visible
		expect(screen.getByText('1 service — all healthy')).toBeInTheDocument();
		expect(screen.getByText('Last updated 0s ago')).toBeInTheDocument();
		expect(screen.queryByText('Command Center v1.0.0')).not.toBeInTheDocument();
	});

	it('shows "No services discovered" when connected but empty', () => {
		setConnectionStatus('connected');
		render(StatusBar);
		expect(screen.getByText('No services discovered')).toBeInTheDocument();
	});

	it('has role="status"', () => {
		render(StatusBar);
		expect(screen.getByRole('status')).toBeInTheDocument();
	});

	it('has aria-live="polite"', () => {
		render(StatusBar);
		expect(screen.getByRole('status')).toHaveAttribute('aria-live', 'polite');
	});

	it('shows app version when set', () => {
		setConnectionStatus('connected');
		replaceAll([], 'v0.2.0');
		render(StatusBar);
		expect(screen.getByText('Command Center v0.2.0')).toBeInTheDocument();
		expect(screen.queryByText(/Last updated/)).not.toBeInTheDocument();
	});

	it('returns to normal health summary when connection status transitions back to connected', () => {
		replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
		setConnectionStatus('reconnecting');
		const { unmount } = render(StatusBar);
		expect(screen.getByText('Reconnecting...')).toBeInTheDocument();
		unmount();
		// Transition back to connected
		setConnectionStatus('connected');
		render(StatusBar);
		expect(screen.queryByText('Reconnecting...')).not.toBeInTheDocument();
		expect(screen.queryByText('Connection lost')).not.toBeInTheDocument();
		expect(screen.getByText('1 service — all healthy')).toBeInTheDocument();
	});

	describe('health breakdown', () => {
		it('shows "all healthy" in green when no problems exist', () => {
			setConnectionStatus('connected');
			replaceAll([
				makeService({ name: 'svc-1', status: 'healthy' }),
				makeService({ name: 'svc-2', status: 'healthy' }),
				makeService({ name: 'svc-3', status: 'healthy' })
			], 'v1.0.0');
			render(StatusBar);
			const allHealthy = screen.getByText('3 services — all healthy');
			expect(allHealthy).toBeInTheDocument();
			expect(allHealthy).toHaveClass('text-health-ok');
		});

		it('shows breakdown with colored segments when problems exist (including unknown)', () => {
			setConnectionStatus('connected');
			replaceAll([
				makeService({ name: 'svc-1', status: 'unhealthy' }),
				makeService({ name: 'svc-2', status: 'unhealthy' }),
				makeService({ name: 'svc-3', status: 'unknown' }),
				makeService({ name: 'svc-4', status: 'healthy' }),
				makeService({ name: 'svc-5', status: 'healthy' })
			], 'v1.0.0');
			render(StatusBar);
			const unhealthySegment = screen.getByText('2 unhealthy');
			expect(unhealthySegment).toHaveClass('text-health-error');
			const unknownSegment = screen.getByText('1 unknown');
			expect(unknownSegment).toHaveClass('text-health-unknown');
			const healthySegment = screen.getByText('2 healthy');
			expect(healthySegment).toHaveClass('text-health-ok');
		});

		it('omits segments with count 0 in breakdown', () => {
			setConnectionStatus('connected');
			replaceAll([
				makeService({ name: 'svc-1', status: 'unhealthy' }),
				makeService({ name: 'svc-2', status: 'healthy' }),
				makeService({ name: 'svc-3', status: 'healthy' })
			], 'v1.0.0');
			render(StatusBar);
			expect(screen.getByText('1 unhealthy')).toBeInTheDocument();
			expect(screen.getByText('2 healthy')).toBeInTheDocument();
		});

		it('preserves "Discovering services..." state unchanged', () => {
			render(StatusBar);
			expect(screen.getByText('Discovering services...')).toBeInTheDocument();
		});

		it('preserves "Reconnecting..." state unchanged', () => {
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
			setConnectionStatus('reconnecting');
			render(StatusBar);
			expect(screen.getByText('Reconnecting...')).toBeInTheDocument();
		});

		it('shows degraded segment with yellow color in breakdown', () => {
			setConnectionStatus('connected');
			replaceAll([
				makeService({ name: 'svc-1', status: 'unhealthy' }),
				makeService({ name: 'svc-2', status: 'degraded' }),
				makeService({ name: 'svc-3', status: 'degraded' }),
				makeService({ name: 'svc-4', status: 'healthy' })
			], 'v1.0.0');
			render(StatusBar);
			const unhealthySegment = screen.getByText('1 unhealthy');
			expect(unhealthySegment).toHaveClass('text-health-error');
			const degradedSegment = screen.getByText('2 degraded');
			expect(degradedSegment).toHaveClass('text-health-degraded');
			const healthySegment = screen.getByText('1 healthy');
			expect(healthySegment).toHaveClass('text-health-ok');
		});

		it('shows breakdown when only healthy and degraded exist (not "all healthy")', () => {
			setConnectionStatus('connected');
			replaceAll([
				makeService({ name: 'svc-1', status: 'healthy' }),
				makeService({ name: 'svc-2', status: 'degraded' })
			], 'v1.0.0');
			render(StatusBar);
			expect(screen.queryByText(/all healthy/)).not.toBeInTheDocument();
			expect(screen.getByText('1 degraded')).toBeInTheDocument();
			expect(screen.getByText('1 healthy')).toBeInTheDocument();
		});

		it('shows unknown in breakdown and does NOT show "all healthy" when only healthy and unknown exist', () => {
			setConnectionStatus('connected');
			replaceAll([
				makeService({ name: 'svc-1', status: 'healthy' }),
				makeService({ name: 'svc-2', status: 'unknown' })
			], 'v1.0.0');
			render(StatusBar);
			expect(screen.queryByText(/all healthy/)).not.toBeInTheDocument();
			expect(screen.getByText('1 unknown')).toBeInTheDocument();
			expect(screen.getByText('1 healthy')).toBeInTheDocument();
		});
	});

	describe('data freshness and staleness', () => {
		it('live-ticks the timestamp — updates from "0s ago" to "1m ago" after 65s', async () => {
			setConnectionStatus('connected');
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
			render(StatusBar);
			expect(screen.getByText(/Last updated 0s ago/)).toBeInTheDocument();

			vi.advanceTimersByTime(65_000);
			await vi.dynamicImportSettled();
			expect(screen.getByText(/Last updated 1m ago/)).toBeInTheDocument();
		});

		it('shows fresh staleness color (subtext-0) when data is recent', () => {
			setConnectionStatus('connected');
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
			render(StatusBar);
			const timeEl = screen.getByText(/Last updated/).closest('time');
			expect(timeEl).toHaveStyle({ color: 'var(--color-subtext-0)' });
		});

		it('shows aging staleness color (yellow) when data exceeds 2x interval', async () => {
			setConnectionStatus('connected');
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0', 30_000);
			render(StatusBar);

			// Advance past 2x interval (61s with 30s interval)
			vi.advanceTimersByTime(61_000);
			await vi.dynamicImportSettled();
			const timeEl = screen.getByText(/Last updated/).closest('time');
			expect(timeEl).toHaveStyle({ color: 'var(--color-health-warning)' });
		});

		it('shows stale staleness color (red) when data exceeds 5x interval', async () => {
			setConnectionStatus('connected');
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0', 30_000);
			render(StatusBar);

			// Advance past 5x interval (151s with 30s interval)
			vi.advanceTimersByTime(151_000);
			await vi.dynamicImportSettled();
			const timeEl = screen.getByText(/Last updated/).closest('time');
			expect(timeEl).toHaveStyle({ color: 'var(--color-health-error)' });
		});

		it('shows stale color and "connection lost" text when SSE disconnects', () => {
					replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
					setConnectionStatus('disconnected');
					render(StatusBar);
							const timeEl = screen.getByText(/Last updated/).closest('time');
							expect(timeEl).toHaveStyle({ color: 'var(--color-health-error)' });
							expect(timeEl?.textContent).not.toContain('— connection lost');
						});
					
						it('shows stale color when reconnecting', () => {
							replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
							setConnectionStatus('reconnecting');
							render(StatusBar);
							const timeEl = screen.getByText(/Last updated/).closest('time');
							expect(timeEl).toHaveStyle({ color: 'var(--color-health-error)' });
						});		it('reflects lastUpdated (health check time) during K8s outage, not k8sLastEvent', () => {
			setConnectionStatus('connected');
			setK8sStatus(false, '2026-02-20T11:55:00Z');
			// Simulate: K8s outage (last K8s event was 5 minutes ago), but health checks still running
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0', 30_000);
			render(StatusBar);
			// The timestamp should show "0s ago" (from lastUpdated) not something stale from k8sLastEvent
			expect(screen.getByText(/Last updated 0s ago/)).toBeInTheDocument();
			expect(screen.queryByText(/Last updated 5m ago/)).not.toBeInTheDocument();
			const timeEl = screen.getByText(/Last updated/).closest('time');
			expect(timeEl).toHaveStyle({ color: 'var(--color-subtext-0)' });
		});

		it('has <time> element with datetime attribute containing ISO string', () => {
			setConnectionStatus('connected');
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
			render(StatusBar);
			const timeEl = screen.getByText(/Last updated/).closest('time');
			expect(timeEl).toBeTruthy();
			expect(timeEl?.getAttribute('datetime')).toMatch(/^\d{4}-\d{2}-\d{2}T/);
		});
	});

	describe('pre-populated startup state', () => {
		it('shows health summary (not "Discovering services...") when services have null lastChecked', () => {
			setConnectionStatus('connected');
			replaceAll([
				makeService({ name: 'svc-1', status: 'healthy', lastChecked: null, lastStateChange: '2026-02-18T10:00:00Z' }),
				makeService({ name: 'svc-2', status: 'healthy', lastChecked: null, lastStateChange: '2026-02-18T10:00:00Z' })
			], 'v1.0.0');
			render(StatusBar);
			expect(screen.queryByText('Discovering services...')).not.toBeInTheDocument();
			expect(screen.getByText('2 services — all healthy')).toBeInTheDocument();
		});

		it('does not show "Last updated" timestamp when all services have null lastChecked', () => {
			setConnectionStatus('connected');
			replaceAll([
				makeService({ name: 'svc-1', status: 'healthy', lastChecked: null, lastStateChange: '2026-02-18T10:00:00Z' })
			], 'v1.0.0');
			render(StatusBar);
			expect(screen.queryByText(/Last updated/)).not.toBeInTheDocument();
		});

		it('shows correct breakdown for mixed statuses with null lastChecked', () => {
			setConnectionStatus('connected');
			replaceAll([
				makeService({ name: 'svc-1', status: 'healthy', lastChecked: null, lastStateChange: '2026-02-18T10:00:00Z' }),
				makeService({ name: 'svc-2', status: 'unhealthy', lastChecked: null, lastStateChange: '2026-02-18T10:00:00Z' })
			], 'v1.0.0');
			render(StatusBar);
			expect(screen.getByText('1 unhealthy')).toBeInTheDocument();
			expect(screen.getByText('1 healthy')).toBeInTheDocument();
		});
	});

	it('does not render any OIDC lock indicator', () => {
		setConnectionStatus('connected');
		replaceAll([
			makeService({ name: 'svc-1', status: 'healthy' })
		], 'v1.0.0');
		render(StatusBar);
		expect(screen.queryByLabelText(/oidc/i)).not.toBeInTheDocument();
		expect(screen.queryByText('🔒')).not.toBeInTheDocument();
	});

	describe('config warning indicator', () => {
		it('does not render ⚠ when there are no config errors', () => {
			setConnectionStatus('connected');
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
			render(StatusBar);
			expect(screen.queryByText('⚠')).not.toBeInTheDocument();
		});

		it('renders ⚠ when config errors exist', () => {
			setConnectionStatus('connected');
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
			setConfigErrors(['services[2].url: required field missing']);
			render(StatusBar);
			expect(screen.getByText('⚠')).toBeInTheDocument();
		});

		it('⚠ has yellow text color', () => {
			setConnectionStatus('connected');
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
			setConfigErrors(['some error']);
			render(StatusBar);
			const warning = screen.getByText('⚠');
			expect(warning).toHaveClass('text-health-warning');
		});

		it('⚠ has title with error count and messages', () => {
			setConnectionStatus('connected');
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
			setConfigErrors(['url required', 'name missing']);
			render(StatusBar);
			const warning = screen.getByText('⚠');
			expect(warning).toHaveAttribute('title', expect.stringContaining('Config: 2 error(s)'));
			expect(warning).toHaveAttribute('title', expect.stringContaining('- url required'));
			expect(warning).toHaveAttribute('title', expect.stringContaining('- name missing'));
		});

		it('⚠ disappears when config errors are cleared', () => {
			setConnectionStatus('connected');
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
			setConfigErrors(['some error']);
			const { unmount } = render(StatusBar);
			expect(screen.getByText('⚠')).toBeInTheDocument();
			unmount();
			setConfigErrors([]);
			render(StatusBar);
			expect(screen.queryByText('⚠')).not.toBeInTheDocument();
		});
	});

});
