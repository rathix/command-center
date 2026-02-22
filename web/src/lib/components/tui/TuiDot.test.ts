import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import TuiDot from './TuiDot.svelte';

describe('TuiDot', () => {
	it('renders with role="img"', () => {
		render(TuiDot, { props: { status: 'unknown' } });
		expect(screen.getByRole('img')).toBeInTheDocument();
	});

	it('renders with aria-label="unknown" when status is unknown', () => {
		render(TuiDot, { props: { status: 'unknown' } });
		expect(screen.getByRole('img')).toHaveAttribute('aria-label', 'unknown');
	});

	it('renders with aria-label="healthy" when status is healthy', () => {
		render(TuiDot, { props: { status: 'healthy' } });
		expect(screen.getByRole('img')).toHaveAttribute('aria-label', 'healthy');
	});

	it('renders with aria-label="unhealthy" when status is unhealthy', () => {
		render(TuiDot, { props: { status: 'unhealthy' } });
		expect(screen.getByRole('img')).toHaveAttribute('aria-label', 'unhealthy');
	});

	it('applies bg-health-ok class for healthy status', () => {
		render(TuiDot, { props: { status: 'healthy' } });
		expect(screen.getByRole('img')).toHaveClass('bg-health-ok');
	});

	it('applies bg-health-error class for unhealthy status', () => {
		render(TuiDot, { props: { status: 'unhealthy' } });
		expect(screen.getByRole('img')).toHaveClass('bg-health-error');
	});

	it('applies bg-health-unknown class for unknown status', () => {
		render(TuiDot, { props: { status: 'unknown' } });
		expect(screen.getByRole('img')).toHaveClass('bg-health-unknown');
	});

	it('renders with aria-label="degraded" when status is degraded', () => {
		render(TuiDot, { props: { status: 'degraded' } });
		expect(screen.getByRole('img')).toHaveAttribute('aria-label', 'degraded');
	});

	it('applies bg-health-degraded class for degraded status', () => {
		render(TuiDot, { props: { status: 'degraded' } });
		expect(screen.getByRole('img')).toHaveClass('bg-health-degraded');
	});
});
