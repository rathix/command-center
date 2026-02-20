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
			makeService({ name: 'svc-1' }),
			makeService({ name: 'svc-2' }),
			makeService({ name: 'svc-3' })
		]);
		render(StatusBar);
		expect(screen.getByText('3 services')).toBeInTheDocument();
	});

	it('shows "Reconnecting..." AND service count when disconnected', () => {
		replaceAll([makeService({ name: 'svc-1' })]);
		setConnectionStatus('disconnected');
		render(StatusBar);
		expect(screen.getByText('Reconnecting...')).toBeInTheDocument();
		expect(screen.getByText('1 services')).toBeInTheDocument();
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
});
