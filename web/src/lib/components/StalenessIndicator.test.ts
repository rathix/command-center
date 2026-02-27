import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import StalenessIndicator from './StalenessIndicator.svelte';
import * as connectivityStore from '$lib/connectivityStore.svelte';

describe('StalenessIndicator', () => {
	beforeEach(() => {
		connectivityStore._resetForTesting();
		vi.restoreAllMocks();
	});

	it('renders nothing when online', () => {
		vi.spyOn(connectivityStore, 'getIsOnline').mockReturnValue(true);
		vi.spyOn(connectivityStore, 'getLastSyncTime').mockReturnValue('2026-02-27T10:00:00Z');

		const { container } = render(StalenessIndicator);
		expect(container.querySelector('[data-testid="staleness-banner"]')).toBeNull();
	});

	it('renders staleness banner when offline with lastSyncTime set', () => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-02-27T10:05:00Z'));

		vi.spyOn(connectivityStore, 'getIsOnline').mockReturnValue(false);
		vi.spyOn(connectivityStore, 'getLastSyncTime').mockReturnValue('2026-02-27T10:00:00Z');

		render(StalenessIndicator);
		const banner = screen.getByTestId('staleness-banner');
		expect(banner).toBeInTheDocument();
		expect(banner.textContent).toContain('Offline');
		expect(banner.textContent).toContain('5 minutes ago');

		vi.useRealTimers();
	});

	it('renders nothing when offline but no lastSyncTime', () => {
		vi.spyOn(connectivityStore, 'getIsOnline').mockReturnValue(false);
		vi.spyOn(connectivityStore, 'getLastSyncTime').mockReturnValue(null);

		const { container } = render(StalenessIndicator);
		expect(container.querySelector('[data-testid="staleness-banner"]')).toBeNull();
	});

	it('shows relative time in hours', () => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-02-27T13:00:00Z'));

		vi.spyOn(connectivityStore, 'getIsOnline').mockReturnValue(false);
		vi.spyOn(connectivityStore, 'getLastSyncTime').mockReturnValue('2026-02-27T10:00:00Z');

		render(StalenessIndicator);
		expect(screen.getByTestId('staleness-banner').textContent).toContain('3 hours ago');

		vi.useRealTimers();
	});

	it('has role="alert" for accessibility', () => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-02-27T10:05:00Z'));

		vi.spyOn(connectivityStore, 'getIsOnline').mockReturnValue(false);
		vi.spyOn(connectivityStore, 'getLastSyncTime').mockReturnValue('2026-02-27T10:00:00Z');

		render(StalenessIndicator);
		expect(screen.getByRole('alert')).toBeInTheDocument();

		vi.useRealTimers();
	});
});
