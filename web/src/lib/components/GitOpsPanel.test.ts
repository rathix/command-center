import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import GitOpsPanel from './GitOpsPanel.svelte';
import { replaceAll, _resetForTesting, setConnectionStatus } from '$lib/serviceStore.svelte';
import type { Service } from '$lib/types';

function makeService(overrides: Partial<Service> & { name: string }): Service {
	const nowIso = new Date().toISOString();
	return {
		displayName: overrides.displayName ?? overrides.name,
		namespace: 'default',
		group: 'default',
		url: 'https://test.example.com',
		status: 'unknown',
		compositeStatus: overrides.compositeStatus ?? overrides.status ?? 'unknown',
		readyEndpoints: null,
		totalEndpoints: null,
		authGuarded: false,
		httpCode: null,
		responseTimeMs: null,
		lastChecked: nowIso,
		lastStateChange: nowIso,
		errorSnippet: null,
		podDiagnostic: null,
		gitopsStatus: null,
		...overrides
	};
}

beforeEach(() => {
	_resetForTesting();
	vi.restoreAllMocks();
});

afterEach(() => {
	vi.restoreAllMocks();
});

describe('GitOpsPanel', () => {
	it('renders "not configured" when status returns configured=false', async () => {
		vi.spyOn(globalThis, 'fetch').mockResolvedValue(
			new Response(JSON.stringify({ ok: true, data: { configured: false } }), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			})
		);

		render(GitOpsPanel);

		await waitFor(() => {
			expect(screen.getByText('GitOps is not configured')).toBeInTheDocument();
		});
	});

	it('renders reconciliation state table when configured with services', async () => {
		vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
			const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
			if (url.includes('/api/gitops/status')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: { configured: true, provider: 'github', repository: 'owner/repo' }
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			if (url.includes('/api/gitops/commits')) {
				return new Response(
					JSON.stringify({ ok: true, data: { commits: [] } }),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			return new Response('{}', { status: 404 });
		});

		setConnectionStatus('connected');
		replaceAll(
			[
				makeService({
					name: 'my-app',
					status: 'healthy',
					gitopsStatus: {
						reconciliationState: 'synced',
						lastTransitionTime: '2026-02-27T10:00:00Z',
						message: 'Applied revision',
						sourceType: 'kustomization'
					}
				})
			],
			'v1.0.0'
		);

		render(GitOpsPanel);

		await waitFor(() => {
			expect(screen.getByText('Reconciliation State')).toBeInTheDocument();
			expect(screen.getByText('synced')).toBeInTheDocument();
			expect(screen.getByText('my-app')).toBeInTheDocument();
		});
	});

	it('renders commit list when commits available', async () => {
		vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
			const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
			if (url.includes('/api/gitops/status')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: { configured: true, provider: 'github', repository: 'owner/repo' }
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			if (url.includes('/api/gitops/commits')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: {
							commits: [
								{
									sha: 'abc1234567890',
									message: 'Update deployment config',
									author: 'Kenny',
									timestamp: '2026-02-27T10:00:00Z'
								}
							]
						}
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			return new Response('{}', { status: 404 });
		});

		render(GitOpsPanel);

		await waitFor(() => {
			expect(screen.getByText('abc1234')).toBeInTheDocument();
			expect(screen.getByText('Update deployment config')).toBeInTheDocument();
			expect(screen.getByText('Kenny')).toBeInTheDocument();
		});
	});

	it('shows rate limit error on 429', async () => {
		vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
			const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
			if (url.includes('/api/gitops/status')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: { configured: true, provider: 'github', repository: 'owner/repo' }
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			if (url.includes('/api/gitops/commits')) {
				return new Response(
					JSON.stringify({ ok: false, error: 'rate limited' }),
					{ status: 429, headers: { 'Content-Type': 'application/json' } }
				);
			}
			return new Response('{}', { status: 404 });
		});

		render(GitOpsPanel);

		await waitFor(() => {
			expect(
				screen.getByText('GitHub API rate limited. Try again later.')
			).toBeInTheDocument();
		});
	});

	it('shows Rollback button on each commit', async () => {
		vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
			const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
			if (url.includes('/api/gitops/status')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: { configured: true, provider: 'github', repository: 'owner/repo' }
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			if (url.includes('/api/gitops/commits')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: {
							commits: [
								{
									sha: 'abc1234567890',
									message: 'Test commit',
									author: 'Kenny',
									timestamp: '2026-02-27T10:00:00Z'
								}
							]
						}
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			return new Response('{}', { status: 404 });
		});

		render(GitOpsPanel);

		await waitFor(() => {
			expect(screen.getByTestId('rollback-button')).toBeInTheDocument();
		});
	});

	it('Rollback button opens confirmation modal with commit details', async () => {
		vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
			const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
			if (url.includes('/api/gitops/status')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: { configured: true, provider: 'github', repository: 'owner/repo' }
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			if (url.includes('/api/gitops/commits')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: {
							commits: [
								{
									sha: 'abc1234567890abcdef',
									message: 'Deploy v2',
									author: 'Kenny',
									timestamp: '2026-02-27T10:00:00Z'
								}
							]
						}
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			return new Response('{}', { status: 404 });
		});

		render(GitOpsPanel);

		await waitFor(() => {
			expect(screen.getByTestId('rollback-button')).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByTestId('rollback-button'));

		await waitFor(() => {
			expect(screen.getByTestId('rollback-modal')).toBeInTheDocument();
			expect(screen.getByText('abc1234567890abcdef')).toBeInTheDocument();
			// Both heading and button have "Confirm Rollback" text
			const matches = screen.getAllByText('Confirm Rollback');
			expect(matches.length).toBeGreaterThanOrEqual(1);
		});
	});

	it('canceling rollback closes modal without API call', async () => {
		const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
			const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
			if (url.includes('/api/gitops/status')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: { configured: true, provider: 'github', repository: 'owner/repo' }
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			if (url.includes('/api/gitops/commits')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: {
							commits: [
								{
									sha: 'abc1234567890',
									message: 'Deploy v2',
									author: 'Kenny',
									timestamp: '2026-02-27T10:00:00Z'
								}
							]
						}
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			return new Response('{}', { status: 404 });
		});

		render(GitOpsPanel);

		await waitFor(() => {
			expect(screen.getByTestId('rollback-button')).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByTestId('rollback-button'));

		await waitFor(() => {
			expect(screen.getByTestId('rollback-modal')).toBeInTheDocument();
		});

		// Click Cancel
		const cancelBtn = screen.getByText('Cancel');
		await fireEvent.click(cancelBtn);

		await waitFor(() => {
			expect(screen.queryByTestId('rollback-modal')).not.toBeInTheDocument();
		});

		// No rollback API call should have been made
		const rollbackCalls = fetchSpy.mock.calls.filter((call) => {
			const url =
				typeof call[0] === 'string'
					? call[0]
					: call[0] instanceof URL
						? call[0].toString()
						: (call[0] as Request).url;
			return url.includes('/api/gitops/rollback');
		});
		expect(rollbackCalls).toHaveLength(0);
	});

	it('confirming rollback sends POST request', async () => {
		vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
			const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
			if (url.includes('/api/gitops/status')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: { configured: true, provider: 'github', repository: 'owner/repo' }
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			if (url.includes('/api/gitops/commits')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: {
							commits: [
								{
									sha: 'abc1234567890',
									message: 'Deploy v2',
									author: 'Kenny',
									timestamp: '2026-02-27T10:00:00Z'
								}
							]
						}
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			if (url.includes('/api/gitops/rollback') && init?.method === 'POST') {
				return new Response(
					JSON.stringify({
						ok: true,
						data: { revertSha: 'revert999', message: 'Revert "Deploy v2"' }
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			return new Response('{}', { status: 404 });
		});

		render(GitOpsPanel);

		await waitFor(() => {
			expect(screen.getByTestId('rollback-button')).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByTestId('rollback-button'));

		await waitFor(() => {
			expect(screen.getByTestId('rollback-modal')).toBeInTheDocument();
		});

		// Click the button, not the heading
		const buttons = screen.getAllByText('Confirm Rollback');
		const confirmBtn = buttons.find((el) => el.tagName === 'BUTTON');
		expect(confirmBtn).toBeTruthy();
		await fireEvent.click(confirmBtn!);

		await waitFor(() => {
			// Modal should close on success
			expect(screen.queryByTestId('rollback-modal')).not.toBeInTheDocument();
		});
	});

	it('highlights correlation between health change and deployment', async () => {
		const recentTime = new Date(Date.now() - 2 * 60 * 1000).toISOString(); // 2 minutes ago

		vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
			const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
			if (url.includes('/api/gitops/status')) {
				return new Response(
					JSON.stringify({
						ok: true,
						data: { configured: true, provider: 'github', repository: 'owner/repo' }
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			if (url.includes('/api/gitops/commits')) {
				return new Response(
					JSON.stringify({ ok: true, data: { commits: [] } }),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				);
			}
			return new Response('{}', { status: 404 });
		});

		setConnectionStatus('connected');
		replaceAll(
			[
				makeService({
					name: 'correlated-app',
					displayName: 'Correlated App',
					status: 'unhealthy',
					lastStateChange: recentTime,
					gitopsStatus: {
						reconciliationState: 'failed',
						lastTransitionTime: recentTime,
						message: 'build failed',
						sourceType: 'kustomization'
					}
				})
			],
			'v1.0.0'
		);

		render(GitOpsPanel);

		await waitFor(() => {
			expect(screen.getByText('Health-Deployment Correlations')).toBeInTheDocument();
			const matches = screen.getAllByText(/Correlated App/);
			expect(matches.length).toBeGreaterThanOrEqual(1);
		});
	});
});
