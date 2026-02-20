import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import SectionLabel from './SectionLabel.svelte';

describe('SectionLabel', () => {
	it('renders the provided label text', () => {
		render(SectionLabel, { props: { label: 'needs attention' } });
		expect(screen.getByText('needs attention')).toBeInTheDocument();
	});

	it('has role="heading" attribute', () => {
		render(SectionLabel, { props: { label: 'needs attention' } });
		expect(screen.getByRole('heading')).toBeInTheDocument();
	});

	it('has aria-level="2" attribute', () => {
		render(SectionLabel, { props: { label: 'healthy' } });
		const heading = screen.getByRole('heading');
		expect(heading).toHaveAttribute('aria-level', '2');
	});

	it('uses text-overlay-1 styling class', () => {
		render(SectionLabel, { props: { label: 'needs attention' } });
		const heading = screen.getByRole('heading');
		expect(heading).toHaveClass('text-overlay-1');
	});

	it('uses text-[11px] and font-semibold classes', () => {
		render(SectionLabel, { props: { label: 'healthy' } });
		const heading = screen.getByRole('heading');
		expect(heading).toHaveClass('text-[11px]');
		expect(heading).toHaveClass('font-semibold');
	});
});
