import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
	fetchNodes,
	fetchMetricsHistory,
	rebootNode,
	upgradeNode,
	fetchUpgradeInfo,
	getNodes,
	getLastPoll,
	getError,
	isStale,
	isConfigured,
	isLoading,
	getMetricsHistory,
	getPendingOperation,
	_resetForTesting
} from './nodeStore.svelte';

beforeEach(() => {
	_resetForTesting();
	vi.restoreAllMocks();
});

describe('nodeStore', () => {
	describe('fetchNodes', () => {
		it('updates nodes state on successful fetch', async () => {
			vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
				new Response(
					JSON.stringify({
						nodes: [
							{ name: 'node-01', health: 'ready', role: 'controlplane', lastSeen: '2026-02-27T12:00:00Z', metrics: null }
						],
						lastPoll: '2026-02-27T12:00:00Z',
						error: null,
						stale: false,
						configured: true
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
			);

			await fetchNodes();

			expect(getNodes()).toHaveLength(1);
			expect(getNodes()[0].name).toBe('node-01');
			expect(isConfigured()).toBe(true);
			expect(isStale()).toBe(false);
			expect(getError()).toBeNull();
			expect(getLastPoll()).toBe('2026-02-27T12:00:00Z');
			expect(isLoading()).toBe(false);
		});

		it('sets stale flag on network error', async () => {
			vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('network down'));

			await fetchNodes();

			expect(isStale()).toBe(true);
			expect(getError()).toBe('network down');
			expect(isLoading()).toBe(false);
		});

		it('sets configured=false when talos not configured', async () => {
			vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
				new Response(
					JSON.stringify({
						nodes: null,
						configured: false
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
			);

			await fetchNodes();

			expect(isConfigured()).toBe(false);
			expect(getNodes()).toHaveLength(0);
		});

		it('parses metrics from response', async () => {
			vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
				new Response(
					JSON.stringify({
						nodes: [
							{
								name: 'node-01',
								health: 'ready',
								role: 'worker',
								lastSeen: '2026-02-27T12:00:00Z',
								metrics: { cpuPercent: 42.5, memoryPercent: 60, diskPercent: 30, timestamp: '2026-02-27T12:00:00Z' }
							}
						],
						lastPoll: '2026-02-27T12:00:00Z',
						error: null,
						stale: false,
						configured: true
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
			);

			await fetchNodes();

			const nodes = getNodes();
			expect(nodes[0].metrics).not.toBeNull();
			expect(nodes[0].metrics!.cpuPercent).toBe(42.5);
		});
	});

	describe('fetchMetricsHistory', () => {
		it('updates metricsHistory map', async () => {
			vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
				new Response(
					JSON.stringify({
						history: [
							{ cpuPercent: 10, memoryPercent: 20, diskPercent: 30, timestamp: '2026-02-27T12:00:00Z' },
							{ cpuPercent: 15, memoryPercent: 25, diskPercent: 35, timestamp: '2026-02-27T12:00:30Z' }
						]
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
			);

			await fetchMetricsHistory('node-01');

			const hist = getMetricsHistory('node-01');
			expect(hist).toHaveLength(2);
			expect(hist[0].cpuPercent).toBe(10);
		});
	});

	describe('rebootNode', () => {
		it('sends POST and returns result', async () => {
			vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
				new Response(
					JSON.stringify({ success: true, message: 'Reboot initiated for node-01' }),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
			);

			const result = await rebootNode('node-01');

			expect(result.success).toBe(true);
			expect(result.message).toContain('Reboot initiated');
			expect(getPendingOperation()).toBeNull(); // cleared after completion
		});
	});

	describe('upgradeNode', () => {
		it('sends POST with targetVersion', async () => {
			const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
				new Response(
					JSON.stringify({ success: true, message: 'Upgrade initiated' }),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
			);

			const result = await upgradeNode('node-01', 'v1.8.0');

			expect(result.success).toBe(true);
			const [url, opts] = fetchSpy.mock.calls[0];
			expect(url).toContain('/api/talos/node-01/upgrade');
			expect(opts?.method).toBe('POST');
			const body = JSON.parse(opts?.body as string);
			expect(body.targetVersion).toBe('v1.8.0');
		});
	});

	describe('fetchUpgradeInfo', () => {
		it('returns version data', async () => {
			vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
				new Response(
					JSON.stringify({ currentVersion: 'v1.7.0', targetVersion: 'v1.8.0' }),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
			);

			const info = await fetchUpgradeInfo('node-01');

			expect(info).not.toBeNull();
			expect(info!.currentVersion).toBe('v1.7.0');
			expect(info!.targetVersion).toBe('v1.8.0');
		});
	});

	describe('pendingOperation', () => {
		it('tracks in-flight operation during reboot', async () => {
			let resolveFetch: (value: Response) => void;
			vi.spyOn(globalThis, 'fetch').mockReturnValueOnce(
				new Promise((resolve) => {
					resolveFetch = resolve;
				})
			);

			const promise = rebootNode('node-01');
			// During the operation, pendingOperation should be set
			expect(getPendingOperation()).toEqual({ node: 'node-01', type: 'reboot' });

			resolveFetch!(
				new Response(JSON.stringify({ success: true }), {
					status: 200,
					headers: { 'Content-Type': 'application/json' }
				})
			);
			await promise;

			expect(getPendingOperation()).toBeNull();
		});
	});

	describe('_resetForTesting', () => {
		it('restores all defaults', async () => {
			// Set some state
			vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
				new Response(
					JSON.stringify({
						nodes: [{ name: 'node-01', health: 'ready', role: 'worker', lastSeen: '2026-02-27T12:00:00Z', metrics: null }],
						lastPoll: '2026-02-27T12:00:00Z',
						error: 'some error',
						stale: true,
						configured: true
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
			);
			await fetchNodes();

			_resetForTesting();

			expect(getNodes()).toHaveLength(0);
			expect(getLastPoll()).toBeNull();
			expect(getError()).toBeNull();
			expect(isStale()).toBe(false);
			expect(isConfigured()).toBe(false);
			expect(isLoading()).toBe(false);
			expect(getMetricsHistory('node-01')).toHaveLength(0);
			expect(getPendingOperation()).toBeNull();
		});
	});
});
