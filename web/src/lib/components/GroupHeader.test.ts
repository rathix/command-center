import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import GroupHeader from './GroupHeader.svelte';
import type { ServiceGroup } from '$lib/types';
import { toggleGroupCollapse } from '$lib/serviceStore.svelte';

vi.mock('$lib/serviceStore.svelte', () => ({
	toggleGroupCollapse: vi.fn()
}));

function makeGroup(overrides: Partial<ServiceGroup> = {}): ServiceGroup {
	return {
		name: 'media',
		services: [],
		counts: { healthy: 7, degraded: 0, unhealthy: 1, unknown: 0 },
		hasProblems: true,
		expanded: true,
		...overrides
	};
}

describe('GroupHeader', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('renders chevron ▾ when group is expanded', () => {
		render(GroupHeader, { props: { group: makeGroup({ expanded: true }), controlsId: 'list-1' } });
		expect(screen.getByText('▾')).toBeInTheDocument();
	});

	it('renders chevron ▸ when group is collapsed', () => {
		render(GroupHeader, { props: { group: makeGroup({ expanded: false }), controlsId: 'list-1' } });
		expect(screen.getByText('▸')).toBeInTheDocument();
	});

	it('renders group name text', () => {
		render(GroupHeader, { props: { group: makeGroup({ name: 'infrastructure' }), controlsId: 'list-1' } });
		expect(screen.getByText('infrastructure')).toBeInTheDocument();
	});

	it('renders health counts with correct text', () => {
		render(GroupHeader, { props: { group: makeGroup(), controlsId: 'list-1' } });
		expect(screen.getByText('7 healthy')).toBeInTheDocument();
		expect(screen.getByText('1 unhealthy')).toBeInTheDocument();
		expect(screen.getByRole('button').textContent).toMatch(/7 healthy,\s*1 unhealthy/);
	});

	it('healthy count uses text-health-ok class', () => {
		render(GroupHeader, { props: { group: makeGroup(), controlsId: 'list-1' } });
		expect(screen.getByText('7 healthy')).toHaveClass('text-health-ok');
	});

	it('unhealthy count uses text-health-error class', () => {
		render(GroupHeader, { props: { group: makeGroup(), controlsId: 'list-1' } });
		expect(screen.getByText('1 unhealthy')).toHaveClass('text-health-error');
	});

	it('has role="button" attribute', () => {
		render(GroupHeader, { props: { group: makeGroup(), controlsId: 'list-1' } });
		expect(screen.getByRole('button')).toBeInTheDocument();
	});

	it('has aria-expanded="true" when expanded', () => {
		render(GroupHeader, { props: { group: makeGroup({ expanded: true }), controlsId: 'list-1' } });
		expect(screen.getByRole('button')).toHaveAttribute('aria-expanded', 'true');
	});

	it('has aria-expanded="false" when collapsed', () => {
		render(GroupHeader, { props: { group: makeGroup({ expanded: false }), controlsId: 'list-1' } });
		expect(screen.getByRole('button')).toHaveAttribute('aria-expanded', 'false');
	});

	it('has aria-controls attribute matching controlsId prop', () => {
		render(GroupHeader, { props: { group: makeGroup(), controlsId: 'group-media-services' } });
		expect(screen.getByRole('button')).toHaveAttribute('aria-controls', 'group-media-services');
	});

	it('has bg-mantle background class', () => {
		render(GroupHeader, { props: { group: makeGroup(), controlsId: 'list-1' } });
		expect(screen.getByRole('button')).toHaveClass('bg-mantle');
	});

	it('has required size, spacing, and focus ring classes', () => {
		render(GroupHeader, { props: { group: makeGroup(), controlsId: 'list-1' } });
		const header = screen.getByRole('button');
		expect(header).toHaveClass('min-h-[var(--touch-target-min)]');
		expect(header).toHaveClass('px-3');
		expect(header).toHaveClass('focus-visible:outline-2');
		expect(header).toHaveClass('focus-visible:outline-offset-[-2px]');
		expect(header).toHaveClass('focus-visible:outline-accent-lavender');
	});

	it('clicking header calls toggleGroupCollapse with group name', async () => {
		render(GroupHeader, { props: { group: makeGroup({ name: 'media' }), controlsId: 'list-1' } });
		await fireEvent.click(screen.getByRole('button'));
		expect(toggleGroupCollapse).toHaveBeenCalledWith('media');
	});

	it('pressing Enter calls toggleGroupCollapse with group name', async () => {
		render(GroupHeader, { props: { group: makeGroup({ name: 'media' }), controlsId: 'list-1' } });
		await fireEvent.keyDown(screen.getByRole('button'), { key: 'Enter' });
		expect(toggleGroupCollapse).toHaveBeenCalledWith('media');
	});

	it('pressing Space calls toggleGroupCollapse with group name', async () => {
		render(GroupHeader, { props: { group: makeGroup({ name: 'media' }), controlsId: 'list-1' } });
		const header = screen.getByRole('button');
		const event = new KeyboardEvent('keydown', { key: ' ', bubbles: true, cancelable: true });
		header.dispatchEvent(event);
		expect(event.defaultPrevented).toBe(true);
		expect(toggleGroupCollapse).toHaveBeenCalledWith('media');
	});

	it('pressing Space (legacy key value) calls toggleGroupCollapse with group name', () => {
		render(GroupHeader, { props: { group: makeGroup({ name: 'media' }), controlsId: 'list-1' } });
		const header = screen.getByRole('button');
		const event = new KeyboardEvent('keydown', { key: 'Space', bubbles: true, cancelable: true });
		header.dispatchEvent(event);
		expect(event.defaultPrevented).toBe(true);
		expect(toggleGroupCollapse).toHaveBeenCalledWith('media');
	});

	it('all-healthy group shows only healthy count', () => {
		render(GroupHeader, {
			props: {
				group: makeGroup({ counts: { healthy: 8, degraded: 0, unhealthy: 0, unknown: 0 } }),
				controlsId: 'list-1'
			}
		});
		expect(screen.getByText('8 healthy')).toBeInTheDocument();
		expect(screen.queryByText(/unhealthy/)).not.toBeInTheDocument();
		expect(screen.queryByText(/unknown/)).not.toBeInTheDocument();
		expect(screen.queryByText(/degraded/)).not.toBeInTheDocument();
	});

	it('degraded count uses text-health-degraded class', () => {
		render(GroupHeader, {
			props: {
				group: makeGroup({ counts: { healthy: 5, degraded: 2, unhealthy: 1, unknown: 0 } }),
				controlsId: 'list-1'
			}
		});
		expect(screen.getByText('2 degraded')).toHaveClass('text-health-degraded');
	});

	it('renders degraded count between healthy and unhealthy', () => {
		render(GroupHeader, {
			props: {
				group: makeGroup({ counts: { healthy: 3, degraded: 1, unhealthy: 1, unknown: 0 } }),
				controlsId: 'list-1'
			}
		});
		const text = screen.getByRole('button').textContent;
		expect(text).toMatch(/3 healthy.*1 degraded.*1 unhealthy/);
	});

	it('hides degraded count when zero', () => {
		render(GroupHeader, {
			props: {
				group: makeGroup({ counts: { healthy: 5, degraded: 0, unhealthy: 1, unknown: 0 } }),
				controlsId: 'list-1'
			}
		});
		expect(screen.queryByText(/degraded/)).not.toBeInTheDocument();
	});
});
