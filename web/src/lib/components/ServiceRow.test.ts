import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ServiceRow from './ServiceRow.svelte';
import type { Service } from '$lib/types';

function makeService(overrides: Partial<Service> = {}): Service {
	return {
		name: 'grafana',
		displayName: 'grafana',
		namespace: 'monitoring',
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

	it('sanitizes unsafe urls to a safe fallback link', () => {
		render(ServiceRow, {
			props: { service: makeService({ url: 'javascript:alert(1)' }), odd: false }
		});
		const link = screen.getByRole('link');
		expect(link).toHaveAttribute('href', '#');
		expect(link).toHaveAttribute('aria-disabled', 'true');
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
});
