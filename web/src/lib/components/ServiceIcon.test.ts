import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ServiceIcon from './ServiceIcon.svelte';

const CDN_BASE = 'https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/svg';

function getImg(container: HTMLElement): HTMLImageElement {
	const img = container.querySelector('img');
	if (!img) throw new Error('No <img> element found');
	return img;
}

describe('ServiceIcon', () => {
	it('renders img with correct CDN URL constructed from name prop', () => {
		const { container } = render(ServiceIcon, { props: { name: 'immich' } });
		const img = getImg(container);
		expect(img).toHaveAttribute('src', `${CDN_BASE}/immich.svg`);
	});

	it('has loading="lazy" attribute', () => {
		const { container } = render(ServiceIcon, { props: { name: 'immich' } });
		const img = getImg(container);
		expect(img).toHaveAttribute('loading', 'lazy');
	});

	it('has aria-hidden="true" attribute', () => {
		const { container } = render(ServiceIcon, { props: { name: 'immich' } });
		const img = getImg(container);
		expect(img).toHaveAttribute('aria-hidden', 'true');
	});

	it('has explicit width, height, and empty alt attributes', () => {
		const { container } = render(ServiceIcon, { props: { name: 'immich' } });
		const img = getImg(container);
		expect(img).toHaveAttribute('width', '16');
		expect(img).toHaveAttribute('height', '16');
		expect(img).toHaveAttribute('alt', '');
	});

	it('renders fallback >_ glyph after error event fires on img', async () => {
		const { container } = render(ServiceIcon, { props: { name: 'nonexistent' } });
		const img = getImg(container);
		await fireEvent.error(img);
		expect(container.querySelector('img')).toBeNull();
		expect(screen.getByText('>_')).toBeInTheDocument();
	});

	it('has initial opacity 0 before load', () => {
		const { container } = render(ServiceIcon, { props: { name: 'immich' } });
		const img = getImg(container);
		expect(img.style.opacity).toBe('0');
	});

	it('transitions to opacity 1 after load event', async () => {
		const { container } = render(ServiceIcon, { props: { name: 'immich' } });
		const img = getImg(container);
		await fireEvent.load(img);
		expect(img.style.opacity).toBe('1');
	});

	it('fallback glyph has aria-hidden="true"', async () => {
		const { container } = render(ServiceIcon, { props: { name: 'nonexistent' } });
		const img = getImg(container);
		await fireEvent.error(img);
		const fallback = screen.getByText('>_');
		expect(fallback).toHaveAttribute('aria-hidden', 'true');
	});

	it('works with hyphenated service names', () => {
		const { container } = render(ServiceIcon, { props: { name: 'home-assistant' } });
		const img = getImg(container);
		expect(img).toHaveAttribute('src', `${CDN_BASE}/home-assistant.svg`);
	});

	it('works with single-word service names', () => {
		const { container } = render(ServiceIcon, { props: { name: 'grafana' } });
		const img = getImg(container);
		expect(img).toHaveAttribute('src', `${CDN_BASE}/grafana.svg`);
	});
});
