import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import NodeDashboard from './NodeDashboard.svelte';
import { _resetForTesting } from '$lib/nodeStore.svelte';

function mockFetchResponse(body: object, status = 200) {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function setupConfiguredNodes(overrides: { stale?: boolean; error?: string | null; nodes?: object[] } = {}) {
	const nodes = overrides.nodes ?? [
		{
			name: 'node-01',
			health: 'ready',
			role: 'controlplane',
			lastSeen: '2026-02-27T12:00:00Z',
			metrics: { cpuPercent: 42, memoryPercent: 60, diskPercent: 30, timestamp: '2026-02-27T12:00:00Z' }
		},
		{
			name: 'node-02',
			health: 'not-ready',
			role: 'worker',
			lastSeen: '2026-02-27T12:00:00Z',
			metrics: null
		}
	];

	const nodesResponse = mockFetchResponse({
		nodes,
		lastPoll: '2026-02-27T12:00:00Z',
		error: overrides.error ?? null,
		stale: overrides.stale ?? false,
		configured: true
	});

	const metricsResponse = mockFetchResponse({ history: [] });

	vi.spyOn(globalThis, 'fetch').mockImplementation((input) => {
		const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
		if (url.includes('/api/nodes/') && url.includes('/metrics')) {
			return Promise.resolve(metricsResponse.clone());
		}
		if (url.includes('/api/nodes')) {
			return Promise.resolve(nodesResponse.clone());
		}
		return Promise.resolve(mockFetchResponse({}));
	});
}

beforeEach(() => {
	_resetForTesting();
	vi.restoreAllMocks();
});

describe('NodeDashboard', () => {
	it('renders "Talos not configured" when configured=false', async () => {
		vi.spyOn(globalThis, 'fetch').mockResolvedValue(
			mockFetchResponse({ nodes: null, configured: false })
		);

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		expect(screen.getByText('Talos not configured')).toBeInTheDocument();
	});

	it('renders node cards with health badges', async () => {
		setupConfiguredNodes();

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		const cards = screen.getAllByTestId('node-card');
		expect(cards).toHaveLength(2);

		expect(screen.getByText('node-01')).toBeInTheDocument();
		expect(screen.getByText('node-02')).toBeInTheDocument();

		const badges = screen.getAllByTestId('health-badge');
		expect(badges).toHaveLength(2);
		expect(badges[0].textContent?.trim()).toBe('ready');
		expect(badges[1].textContent?.trim()).toBe('not-ready');
	});

	it('renders role tags', async () => {
		setupConfiguredNodes();

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		const roleTags = screen.getAllByTestId('role-tag');
		expect(roleTags[0].textContent?.trim()).toBe('controlplane');
		expect(roleTags[1].textContent?.trim()).toBe('worker');
	});

	it('renders staleness indicator when stale=true', async () => {
		setupConfiguredNodes({ stale: true });

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		expect(screen.getByTestId('staleness-indicator')).toBeInTheDocument();
		expect(screen.getByText('Node data may be stale')).toBeInTheDocument();
	});

	it('renders error message', async () => {
		setupConfiguredNodes({ error: 'connection refused' });

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		expect(screen.getByTestId('error-message')).toBeInTheDocument();
		expect(screen.getByText('connection refused')).toBeInTheDocument();
	});

	it('renders CPU, memory, disk percentages per node', async () => {
		setupConfiguredNodes();

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		expect(screen.getByTestId('cpu-percent').textContent?.trim()).toBe('42%');
		expect(screen.getByTestId('memory-percent').textContent?.trim()).toBe('60%');
		expect(screen.getByTestId('disk-percent').textContent?.trim()).toBe('30%');
	});

	it('renders "Metrics unavailable" when metrics null', async () => {
		setupConfiguredNodes();

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		expect(screen.getByTestId('metrics-unavailable')).toBeInTheDocument();
	});

	it('renders sparkline components', async () => {
		setupConfiguredNodes();

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		// Sparklines are rendered for the node with metrics
		const metricsSection = screen.getByTestId('metrics-section');
		expect(metricsSection).toBeInTheDocument();
	});

	it('buttons disabled when stale', async () => {
		setupConfiguredNodes({ stale: true });

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		const rebootBtns = screen.getAllByTestId('reboot-btn');
		const upgradeBtns = screen.getAllByTestId('upgrade-btn');
		for (const btn of [...rebootBtns, ...upgradeBtns]) {
			expect(btn).toBeDisabled();
		}
	});

	it('Reboot button opens ConfirmationModal with node details', async () => {
		setupConfiguredNodes({ nodes: [
			{ name: 'node-01', health: 'ready', role: 'worker', lastSeen: '2026-02-27T12:00:00Z', metrics: null }
		]});

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		const rebootBtn = screen.getByTestId('reboot-btn');
		await fireEvent.click(rebootBtn);

		expect(screen.getByRole('dialog')).toBeInTheDocument();
		expect(screen.getByText('Reboot node-01?')).toBeInTheDocument();
	});

	it('Upgrade button fetches upgrade info then opens modal', async () => {
		const fetchSpy = vi.spyOn(globalThis, 'fetch');
		// First call: /api/nodes
		fetchSpy.mockImplementation((input) => {
			const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
			if (url.includes('/upgrade-info')) {
				return Promise.resolve(
					mockFetchResponse({ currentVersion: 'v1.7.0', targetVersion: 'v1.8.0' })
				);
			}
			if (url.includes('/api/nodes/') && url.includes('/metrics')) {
				return Promise.resolve(mockFetchResponse({ history: [] }));
			}
			if (url.includes('/api/nodes')) {
				return Promise.resolve(
					mockFetchResponse({
						nodes: [{ name: 'node-01', health: 'ready', role: 'worker', lastSeen: '2026-02-27T12:00:00Z', metrics: null }],
						lastPoll: '2026-02-27T12:00:00Z',
						error: null,
						stale: false,
						configured: true
					})
				);
			}
			return Promise.resolve(mockFetchResponse({}));
		});

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		const upgradeBtn = screen.getByTestId('upgrade-btn');
		await fireEvent.click(upgradeBtn);
		await vi.dynamicImportSettled();

		expect(screen.getByRole('dialog')).toBeInTheDocument();
		expect(screen.getByText('Upgrade node-01?')).toBeInTheDocument();
	});

	it('color-codes percentages by threshold', async () => {
		setupConfiguredNodes({
			nodes: [
				{
					name: 'node-01',
					health: 'ready',
					role: 'worker',
					lastSeen: '2026-02-27T12:00:00Z',
					metrics: { cpuPercent: 90, memoryPercent: 70, diskPercent: 30, timestamp: '2026-02-27T12:00:00Z' }
				}
			]
		});

		render(NodeDashboard);
		await vi.dynamicImportSettled();

		const cpuEl = screen.getByTestId('cpu-percent');
		const memEl = screen.getByTestId('memory-percent');
		const diskEl = screen.getByTestId('disk-percent');

		expect(cpuEl).toHaveClass('text-red'); // > 85%
		expect(memEl).toHaveClass('text-yellow'); // 60-85%
		expect(diskEl).toHaveClass('text-green'); // < 60%
	});
});
