import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, beforeEach } from 'vitest';
import StatusBar from './StatusBar.svelte';
import {
	replaceAll,
	setConnectionStatus,
	_resetForTesting
} from '$lib/serviceStore.svelte';
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

	it('shows "Reconnecting..." AND service count when disconnected', () => {
		replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
		setConnectionStatus('disconnected');
		render(StatusBar);
		expect(screen.getByText('Reconnecting...')).toBeInTheDocument();
		expect(screen.getByText('1 services — all healthy')).toBeInTheDocument();
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
				makeService({ name: 'svc-3', status: 'authBlocked' }),
				makeService({ name: 'svc-4', status: 'unknown' }),
				makeService({ name: 'svc-5', status: 'healthy' }),
				makeService({ name: 'svc-6', status: 'healthy' })
			], 'v1.0.0');
			render(StatusBar);
			const unhealthySegment = screen.getByText('2 unhealthy');
			expect(unhealthySegment).toHaveClass('text-health-error');
			const authBlockedSegment = screen.getByText('1 auth-blocked');
			expect(authBlockedSegment).toHaveClass('text-health-auth-blocked');
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
			expect(screen.queryByText(/auth-blocked/)).not.toBeInTheDocument();
		});

		it('preserves "Discovering services..." state unchanged', () => {
			render(StatusBar);
			expect(screen.getByText('Discovering services...')).toBeInTheDocument();
		});

		it('preserves "Reconnecting..." state unchanged', () => {
			replaceAll([makeService({ name: 'svc-1', status: 'healthy' })], 'v1.0.0');
			setConnectionStatus('disconnected');
			render(StatusBar);
			expect(screen.getByText('Reconnecting...')).toBeInTheDocument();
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
});
