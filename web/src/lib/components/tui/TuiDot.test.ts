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

	it('renders with aria-label="authBlocked" when status is authBlocked', () => {
		render(TuiDot, { props: { status: 'authBlocked' } });
		expect(screen.getByRole('img')).toHaveAttribute('aria-label', 'authBlocked');
	});

	it('applies bg-health-ok class for healthy status', () => {
		render(TuiDot, { props: { status: 'healthy' } });
		expect(screen.getByRole('img')).toHaveClass('bg-health-ok');
	});

	it('applies bg-health-error class for unhealthy status', () => {
		render(TuiDot, { props: { status: 'unhealthy' } });
		expect(screen.getByRole('img')).toHaveClass('bg-health-error');
	});

	it('applies bg-health-auth-blocked class for authBlocked status', () => {
		render(TuiDot, { props: { status: 'authBlocked' } });
		expect(screen.getByRole('img')).toHaveClass('bg-health-auth-blocked');
	});

	it('applies bg-health-unknown class for unknown status', () => {
		render(TuiDot, { props: { status: 'unknown' } });
		expect(screen.getByRole('img')).toHaveClass('bg-health-unknown');
	});
});
