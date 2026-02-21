import { render, screen, fireEvent } from '@testing-library/svelte';
import { tick } from 'svelte';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import ServiceRow from './ServiceRow.svelte';
import type { Service } from '$lib/types';

function makeService(overrides: Partial<Service> = {}): Service {
	return {
		name: 'grafana',
		displayName: 'grafana',
		namespace: 'monitoring',
		group: 'monitoring',
		url: 'https://grafana.example.com',
		status: 'unknown',
		httpCode: null,
		responseTimeMs: null,
		lastChecked: null,
		lastStateChange: null,
		errorSnippet: null,
		...overrides
	};
}

describe('ServiceRow', () => {
	it('renders service display name text', () => {
		render(ServiceRow, {
			props: { service: makeService({ displayName: 'my-service' }), odd: false }
		});
		expect(screen.getByText('my-service')).toBeInTheDocument();
	});

	it('renders service URL text', () => {
		render(ServiceRow, {
			props: { service: makeService({ url: 'https://svc.example.com' }), odd: false }
		});
		expect(screen.getByText('https://svc.example.com')).toBeInTheDocument();
	});

	it('renders as <a> with correct href', () => {
		render(ServiceRow, {
			props: { service: makeService({ url: 'https://grafana.example.com' }), odd: false }
		});
		const link = screen.getByRole('link');
		expect(link.tagName).toBe('A');
		expect(link).toHaveAttribute('href', 'https://grafana.example.com/');
	});

	it('has target="_blank" and rel="noopener noreferrer"', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const link = screen.getByRole('link');
		expect(link).toHaveAttribute('target', '_blank');
		expect(link).toHaveAttribute('rel', 'noopener noreferrer');
	});

	it('renders TuiDot with service status', () => {
		render(ServiceRow, {
			props: { service: makeService({ status: 'unknown' }), odd: false }
		});
		const dot = screen.getByRole('img');
		expect(dot).toHaveAttribute('aria-label', 'unknown');
	});

	it('has bg-surface-0 class when odd=true', () => {
		render(ServiceRow, { props: { service: makeService(), odd: true } });
		const listItem = screen.getByRole('listitem');
		expect(listItem).toHaveClass('bg-surface-0');
	});

	it('does not have bg-surface-0 class when odd=false', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const listItem = screen.getByRole('listitem');
		expect(listItem).not.toHaveClass('bg-surface-0');
	});

	it('has role="listitem"', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		expect(screen.getByRole('listitem')).toBeInTheDocument();
	});

	it('has hover transition classes', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const listItem = screen.getByRole('listitem');
		expect(listItem).toHaveClass('transition-colors');
		expect(listItem).toHaveClass('duration-300');
	});

	it('has cursor-pointer class', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const link = screen.getByRole('link');
		expect(link).toHaveClass('cursor-pointer');
	});

	it('has fixed 46px height', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const listItem = screen.getByRole('listitem');
		expect(listItem).toHaveClass('h-[46px]');
	});

	it('renders non-interactive div for unsafe urls', () => {
		render(ServiceRow, {
			props: { service: makeService({ url: 'javascript:alert(1)' }), odd: false }
		});
		const div = screen.getByTitle('Invalid URL');
		expect(div).toBeInTheDocument();
		expect(screen.queryByRole('link')).toBeNull();
		expect(screen.getByText(/javascript:alert\(1\) \(invalid\)/)).toBeInTheDocument();
	});

	it('uses service icon override for icon src in link branch', () => {
		const { container } = render(ServiceRow, {
			props: {
				service: makeService({ icon: 'immich' }),
				odd: false
			}
		});
		const iconImg = container.querySelector('img[loading="lazy"]');
		expect(iconImg).toHaveAttribute(
			'src',
			'https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/svg/immich.svg'
		);
	});

	it('uses displayName as icon fallback when icon override is empty', () => {
		const { container } = render(ServiceRow, {
			props: {
				service: makeService({ name: 'grafana-ingress', displayName: 'grafana', icon: '  ' }),
				odd: false
			}
		});
		const iconImg = container.querySelector('img[loading="lazy"]');
		expect(iconImg).toHaveAttribute(
			'src',
			'https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/svg/grafana.svg'
		);
	});

	it('uses service icon override for icon src in invalid URL branch', () => {
		const { container } = render(ServiceRow, {
			props: {
				service: makeService({ url: 'javascript:alert(1)', icon: 'vaultwarden' }),
				odd: false
			}
		});
		const iconImg = container.querySelector('img[loading="lazy"]');
		expect(iconImg).toHaveAttribute(
			'src',
			'https://cdn.jsdelivr.net/gh/walkxcode/dashboard-icons/svg/vaultwarden.svg'
		);
	});

	describe('OIDC lock glyph', () => {
		it('renders lock with aria-label when authMethod is "oidc"', () => {
			render(ServiceRow, {
				props: { service: makeService({ authMethod: 'oidc' }), odd: false }
			});
			expect(screen.getByLabelText('OIDC authenticated')).toBeInTheDocument();
		});

		it('does not render lock when authMethod is undefined', () => {
			render(ServiceRow, {
				props: { service: makeService(), odd: false }
			});
			expect(screen.queryByLabelText('OIDC authenticated')).not.toBeInTheDocument();
		});

		it('renders lock in invalid URL branch too', () => {
			render(ServiceRow, {
				props: { service: makeService({ authMethod: 'oidc', url: 'javascript:alert(1)' }), odd: false }
			});
			expect(screen.getByLabelText('OIDC authenticated')).toBeInTheDocument();
		});

		it('lock glyph appears between icon and display name (DOM order)', () => {
			const { container } = render(ServiceRow, {
				props: { service: makeService({ authMethod: 'oidc', displayName: 'my-svc' }), odd: false }
			});
			const lockEl = screen.getByLabelText('OIDC authenticated');
			const nameEl = screen.getByText('my-svc');
			// Lock should come before name in DOM
			const allSpans = Array.from(container.querySelectorAll('span'));
			const lockIdx = allSpans.indexOf(lockEl as HTMLSpanElement);
			const nameIdx = allSpans.indexOf(nameEl as HTMLSpanElement);
			expect(lockIdx).toBeLessThan(nameIdx);
			expect(lockIdx).toBeGreaterThan(-1);
		});
	});

	describe('source glyph', () => {
		it('renders ⎈ glyph for kubernetes source', () => {
			render(ServiceRow, {
				props: { service: makeService({ source: 'kubernetes' }), odd: false }
			});
			expect(screen.getByText('⎈')).toBeInTheDocument();
		});

		it('renders ⌂ glyph for config source', () => {
			render(ServiceRow, {
				props: { service: makeService({ source: 'config' }), odd: false }
			});
			expect(screen.getByText('⌂')).toBeInTheDocument();
		});

		it('renders no source glyph when source is undefined', () => {
			render(ServiceRow, {
				props: { service: makeService(), odd: false }
			});
			expect(screen.queryByText('⎈')).not.toBeInTheDocument();
			expect(screen.queryByText('⌂')).not.toBeInTheDocument();
		});

		it('source glyph has aria-hidden="true"', () => {
			render(ServiceRow, {
				props: { service: makeService({ source: 'kubernetes' }), odd: false }
			});
			expect(screen.getByText('⎈')).toHaveAttribute('aria-hidden', 'true');
		});
	});

	describe('health response details', () => {
		it('shows response code and time for healthy service in muted text', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'healthy', httpCode: 200, responseTimeMs: 142 }),
					odd: false
				}
			});
			const responseText = screen.getByText('200 · 142ms');
			expect(responseText).toBeInTheDocument();
			expect(responseText).toHaveClass('text-subtext-0');
		});

		it('shows response code and time for unhealthy service in red text', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'unhealthy', httpCode: 503, responseTimeMs: 0 }),
					odd: false
				}
			});
			const responseText = screen.getByText('503 · 0ms');
			expect(responseText).toBeInTheDocument();
			expect(responseText).toHaveClass('text-health-error');
		});

		it('shows response code and time for authBlocked service in yellow text', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'authBlocked', httpCode: 401, responseTimeMs: 24 }),
					odd: false
				}
			});
			const responseText = screen.getByText('401 · 24ms');
			expect(responseText).toBeInTheDocument();
			expect(responseText).toHaveClass('text-health-auth-blocked');
		});

		it('shows dash placeholder for unknown status and uses semantic color', () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						status: 'unknown',
						httpCode: 200,
						responseTimeMs: 50
					}),
					odd: false
				}
			});
			const responseText = screen.getByText('—');
			expect(responseText).toBeInTheDocument();
			expect(responseText).toHaveClass('text-health-unknown');
		});

		it('shows dash placeholder when httpCode is null', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'healthy', httpCode: null, responseTimeMs: 100 }),
					odd: false
				}
			});
			expect(screen.getByText('—')).toBeInTheDocument();
		});

		it('shows dash placeholder when responseTimeMs is null', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'healthy', httpCode: 200, responseTimeMs: null }),
					odd: false
				}
			});
			expect(screen.getByText('—')).toBeInTheDocument();
		});
	});

	describe('row background tinting', () => {
		it('has faint red background tint for unhealthy service', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'unhealthy', httpCode: 503, responseTimeMs: 0 }),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');
			expect(listItem.style.backgroundColor).toBe('rgba(243, 139, 168, 0.05)');
		});

		it('has faint yellow background tint for authBlocked service', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'authBlocked', httpCode: 401, responseTimeMs: 24 }),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');
			expect(listItem.style.backgroundColor).toBe('rgba(249, 226, 175, 0.03)');
		});

		it('has no background tint for healthy service', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'healthy', httpCode: 200, responseTimeMs: 142 }),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');
			expect(listItem.style.backgroundColor).toBe('');
		});

		it('has no background tint for unknown service', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'unknown' }),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');
			expect(listItem.style.backgroundColor).toBe('');
		});

		it('preserves bg-surface-0 on odd rows alongside unhealthy tint', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'unhealthy', httpCode: 503, responseTimeMs: 0 }),
					odd: true
				}
			});
			const listItem = screen.getByRole('listitem');
			expect(listItem).toHaveClass('bg-surface-0');
			expect(listItem.style.backgroundColor).toBe('rgba(243, 139, 168, 0.05)');
		});
	});

	describe('hover tooltip integration', () => {
		beforeEach(() => {
			vi.useFakeTimers();
			vi.setSystemTime(new Date('2026-02-20T10:00:12Z'));
		});

		afterEach(() => {
			vi.useRealTimers();
		});

		it('tooltip is NOT visible by default', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ lastChecked: '2026-02-20T10:00:00Z' }),
					odd: false
				}
			});
			expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
		});

		it('aria-describedby is set on the <a> element with the correct tooltip id', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ name: 'grafana', namespace: 'monitoring' }),
					odd: false
				}
			});
			const link = screen.getByRole('link');
			expect(link).toHaveAttribute('aria-describedby', 'tooltip-monitoring-grafana');
		});

		it('tooltip appears after mouseenter + 200ms delay', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						lastChecked: '2026-02-20T10:00:00Z',
						lastStateChange: '2026-02-20T08:00:00Z',
						status: 'healthy'
					}),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');

			await fireEvent.mouseEnter(listItem);
			expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();

			vi.advanceTimersByTime(200);
			await tick();
			expect(screen.getByRole('tooltip')).toBeInTheDocument();
		});

		it('positions tooltip above when row is near viewport bottom', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						lastChecked: '2026-02-20T10:00:00Z',
						lastStateChange: '2026-02-20T08:00:00Z',
						status: 'healthy'
					}),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');
			const originalInnerHeight = window.innerHeight;

			Object.defineProperty(window, 'innerHeight', {
				configurable: true,
				value: 800
			});
			vi.spyOn(listItem, 'getBoundingClientRect').mockReturnValue({
				x: 0,
				y: 750,
				top: 750,
				bottom: 790,
				left: 0,
				right: 600,
				width: 600,
				height: 40,
				toJSON: () => ({})
			} as DOMRect);

			try {
				await fireEvent.mouseEnter(listItem);
				vi.advanceTimersByTime(200);
				await tick();

				const tooltip = screen.getByRole('tooltip');
				expect(tooltip).toHaveClass('bottom-full');
				expect(tooltip).toHaveClass('mb-1');
			} finally {
				Object.defineProperty(window, 'innerHeight', {
					configurable: true,
					value: originalInnerHeight
				});
			}
		});

		it('tooltip disappears immediately on mouseleave', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						lastChecked: '2026-02-20T10:00:00Z',
						lastStateChange: '2026-02-20T08:00:00Z',
						status: 'healthy'
					}),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');

			await fireEvent.mouseEnter(listItem);
			vi.advanceTimersByTime(200);
			await tick();
			expect(screen.getByRole('tooltip')).toBeInTheDocument();

			await fireEvent.mouseLeave(listItem);
			await tick();
			expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
		});

		it('tooltip does NOT appear if mouse leaves before 200ms delay', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						lastChecked: '2026-02-20T10:00:00Z',
						lastStateChange: '2026-02-20T08:00:00Z',
						status: 'healthy'
					}),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');

			await fireEvent.mouseEnter(listItem);
			vi.advanceTimersByTime(100);
			await fireEvent.mouseLeave(listItem);
			await tick();
			vi.advanceTimersByTime(200);
			await tick();

			expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
		});

		it('horizontally positions tooltip based on mouse clientX', async () => {
			render(ServiceRow, {
				props: {
					service: makeService(),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');

			// Mock getBoundingClientRect for the row
			vi.spyOn(listItem, 'getBoundingClientRect').mockReturnValue({
				left: 100,
				top: 100,
				bottom: 146,
				right: 700,
				width: 600,
				height: 46
			} as DOMRect);

			// Mouse enters at clientX = 250
			// Expected left = 250 - 100 = 150
			await fireEvent.mouseEnter(listItem, { clientX: 250 });
			vi.advanceTimersByTime(200);
			await tick();

			const tooltip = screen.getByRole('tooltip');
			expect(tooltip.style.left).toBe('150px');
		});

		it('tooltip renders service diagnostic data', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						lastChecked: '2026-02-20T10:00:00Z',
						lastStateChange: '2026-02-20T08:00:00Z',
						status: 'healthy'
					}),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');

			await fireEvent.mouseEnter(listItem);
			vi.advanceTimersByTime(200);
			await tick();

			expect(screen.getByText(/healthy for 2h/)).toBeInTheDocument();
		});
	});
});
