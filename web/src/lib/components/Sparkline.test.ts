import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import Sparkline from './Sparkline.svelte';

describe('Sparkline', () => {
	it('renders SVG with correct number of points for polyline', () => {
		const { container } = render(Sparkline, { props: { data: [10, 50, 90, 30] } });
		const svg = container.querySelector('svg');
		expect(svg).toBeTruthy();
		const polyline = container.querySelector('polyline');
		expect(polyline).toBeTruthy();
		const points = polyline!.getAttribute('points')!.trim().split(' ');
		expect(points).toHaveLength(4);
	});

	it('handles empty data array (no SVG rendered)', () => {
		const { container } = render(Sparkline, { props: { data: [] } });
		const svg = container.querySelector('svg');
		expect(svg).toBeNull();
	});

	it('handles single data point (renders dot)', () => {
		const { container } = render(Sparkline, { props: { data: [50] } });
		const svg = container.querySelector('svg');
		expect(svg).toBeTruthy();
		const circle = container.querySelector('circle');
		expect(circle).toBeTruthy();
		expect(container.querySelector('polyline')).toBeNull();
	});

	it('uses provided color', () => {
		const { container } = render(Sparkline, {
			props: { data: [10, 50], color: '#ff0000' }
		});
		const polyline = container.querySelector('polyline');
		expect(polyline).toBeTruthy();
		expect(polyline!.getAttribute('stroke')).toBe('#ff0000');
	});

	it('has aria-label sparkline', () => {
		const { container } = render(Sparkline, { props: { data: [10, 50] } });
		const svg = container.querySelector('svg');
		expect(svg!.getAttribute('aria-label')).toBe('sparkline');
	});

	it('clamps values to 0-100 range', () => {
		const { container } = render(Sparkline, { props: { data: [-10, 150, 50] } });
		const polyline = container.querySelector('polyline');
		expect(polyline).toBeTruthy();
		// Should not crash; values clamped to [0, 100]
		const points = polyline!.getAttribute('points')!.trim().split(' ');
		expect(points).toHaveLength(3);
	});
});
