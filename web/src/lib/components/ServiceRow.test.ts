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
		compositeStatus: overrides.compositeStatus ?? overrides.status ?? 'unknown',
		readyEndpoints: null,
		totalEndpoints: null,
		authGuarded: false,
		httpCode: null,
		responseTimeMs: null,
		lastChecked: null,
		lastStateChange: null,
		errorSnippet: null,
		podDiagnostic: null,
		gitopsStatus: null,
		...overrides
	} as Service;
}

describe('ServiceRow', () => {
	it('renders service display name text', () => {
		render(ServiceRow, {
			props: { service: makeService({ displayName: 'my-service' }), odd: false }
		});
		expect(screen.getByText('my-service')).toBeInTheDocument();
	});

	it('renders as interactive row with button role', () => {
		render(ServiceRow, {
			props: { service: makeService({ url: 'https://grafana.example.com' }), odd: false }
		});
		const button = screen.getByRole('button');
		expect(button).toBeInTheDocument();
	});

	it('has target="_blank" and rel="noopener noreferrer" on expanded link', async () => {
		render(ServiceRow, {
			props: { service: makeService({ lastChecked: '2026-02-20T10:00:00Z' }), odd: false }
		});
		const button = screen.getByRole('button');
		await fireEvent.click(button);
		await tick();
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
		const button = screen.getByRole('button');
		expect(button).toHaveClass('cursor-pointer');
	});

	it('has minimum service row height for touch targets', () => {
		render(ServiceRow, { props: { service: makeService(), odd: false } });
		const listItem = screen.getByRole('listitem');
		expect(listItem).toHaveClass('min-h-[var(--service-row-min-height)]');
	});

	it('renders non-interactive div for unsafe urls', () => {
		render(ServiceRow, {
			props: { service: makeService({ url: 'javascript:alert(1)' }), odd: false }
		});
		const div = screen.getByTitle('Invalid URL');
		expect(div).toBeInTheDocument();
	});

	it('uses service icon override for icon src', () => {
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
		it('uses compositeStatus for visual state when raw status differs', () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						status: 'healthy',
						compositeStatus: 'unhealthy',
						httpCode: 503,
						responseTimeMs: 0
					}),
					odd: false
				}
			});
			expect(screen.getByLabelText('unhealthy')).toBeInTheDocument();
			const listItem = screen.getByRole('listitem');
			expect(listItem.style.backgroundColor).toBe('rgba(243, 139, 168, 0.05)');
		});
	});

	describe('row background tinting', () => {
		it('has faint red background tint for unhealthy service', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'unhealthy', compositeStatus: 'unhealthy', httpCode: 503, responseTimeMs: 0 }),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');
			expect(listItem.style.backgroundColor).toBe('rgba(243, 139, 168, 0.05)');
		});

		it('has faint yellow background tint for degraded service', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'degraded', compositeStatus: 'degraded', httpCode: 200, responseTimeMs: 142 }),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');
			expect(listItem.style.backgroundColor).toBe('rgba(249, 226, 175, 0.03)');
		});

		it('has no background tint for healthy service', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'healthy', compositeStatus: 'healthy', httpCode: 200, responseTimeMs: 142 }),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');
			expect(listItem.style.backgroundColor).toBe('');
		});

		it('has no background tint for unknown service', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'unknown', compositeStatus: 'unknown' }),
					odd: false
				}
			});
			const listItem = screen.getByRole('listitem');
			expect(listItem.style.backgroundColor).toBe('');
		});

		it('preserves bg-surface-0 on odd rows alongside unhealthy tint', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ status: 'unhealthy', compositeStatus: 'unhealthy', httpCode: 503, responseTimeMs: 0 }),
					odd: true
				}
			});
			const listItem = screen.getByRole('listitem');
			expect(listItem).toHaveClass('bg-surface-0');
			expect(listItem.style.backgroundColor).toBe('rgba(243, 139, 168, 0.05)');
		});
	});

	describe('auth-guarded shield glyph', () => {
		it('shows shield glyph when authGuarded is true', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ authGuarded: true, source: 'kubernetes' }),
					odd: false
				}
			});
			expect(screen.getByText('🛡')).toBeInTheDocument();
		});

		it('does not show shield glyph when authGuarded is false', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ authGuarded: false, source: 'kubernetes' }),
					odd: false
				}
			});
			expect(screen.queryByText('🛡')).not.toBeInTheDocument();
		});

		it('shield glyph has aria-hidden="true"', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ authGuarded: true, source: 'kubernetes' }),
					odd: false
				}
			});
			expect(screen.getByText('🛡')).toHaveAttribute('aria-hidden', 'true');
		});
	});

	describe('expandable details', () => {
		it('clicking row toggles expanded details', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						url: 'https://grafana.example.com',
						httpCode: 200,
						responseTimeMs: 50,
						lastChecked: '2026-02-20T10:00:00Z'
					}),
					odd: false
				}
			});
			const button = screen.getByRole('button');
			expect(screen.queryByTestId('expanded-details')).not.toBeInTheDocument();

			await fireEvent.click(button);
			await tick();
			expect(screen.getByTestId('expanded-details')).toBeInTheDocument();

			await fireEvent.click(button);
			await tick();
			expect(screen.queryByTestId('expanded-details')).not.toBeInTheDocument();
		});

		it('expanded section shows service URL', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						url: 'https://grafana.example.com',
						httpCode: 200,
						responseTimeMs: 50
					}),
					odd: false
				}
			});
			await fireEvent.click(screen.getByRole('button'));
			await tick();
			const link = screen.getByRole('link');
			expect(link).toHaveAttribute('href', 'https://grafana.example.com/');
		});

		it('expanded section shows HTTP code', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({ httpCode: 200, responseTimeMs: 50 }),
					odd: false
				}
			});
			await fireEvent.click(screen.getByRole('button'));
			await tick();
			expect(screen.getByText('200')).toBeInTheDocument();
		});

		it('expanded section shows response time', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({ httpCode: 200, responseTimeMs: 142 }),
					odd: false
				}
			});
			await fireEvent.click(screen.getByRole('button'));
			await tick();
			expect(screen.getByText('142ms')).toBeInTheDocument();
		});

		it('expanded section shows error snippet', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({ errorSnippet: 'Connection refused' }),
					odd: false
				}
			});
			await fireEvent.click(screen.getByRole('button'));
			await tick();
			expect(screen.getByText('Connection refused')).toBeInTheDocument();
		});

		it('has aria-expanded attribute', () => {
			render(ServiceRow, {
				props: { service: makeService(), odd: false }
			});
			const button = screen.getByRole('button');
			expect(button).toHaveAttribute('aria-expanded', 'false');
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

		it('aria-describedby is set on the interactive element with the correct tooltip id', () => {
			render(ServiceRow, {
				props: {
					service: makeService({ name: 'grafana', namespace: 'monitoring' }),
					odd: false
				}
			});
			const button = screen.getByRole('button');
			expect(button).toHaveAttribute('aria-describedby', 'tooltip-monitoring-grafana');
		});

		it('tooltip appears after mouseenter + 200ms delay', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						lastChecked: '2026-02-20T10:00:00Z',
						lastStateChange: '2026-02-20T08:00:00Z',
						status: 'healthy',
						compositeStatus: 'healthy'
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

		it('tooltip disappears immediately on mouseleave', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						lastChecked: '2026-02-20T10:00:00Z',
						lastStateChange: '2026-02-20T08:00:00Z',
						status: 'healthy',
						compositeStatus: 'healthy'
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
						status: 'healthy',
						compositeStatus: 'healthy'
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

		it('tooltip renders service diagnostic data', async () => {
			render(ServiceRow, {
				props: {
					service: makeService({
						lastChecked: '2026-02-20T10:00:00Z',
						lastStateChange: '2026-02-20T08:00:00Z',
						status: 'healthy',
						compositeStatus: 'healthy'
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
