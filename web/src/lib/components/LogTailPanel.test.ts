import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import LogTailPanel from './LogTailPanel.svelte';

// Mock xterm.js with proper class constructors
vi.mock('@xterm/xterm', () => {
	class MockTerminal {
		open = vi.fn();
		write = vi.fn();
		dispose = vi.fn();
		clear = vi.fn();
		onScroll = vi.fn();
		scrollToBottom = vi.fn();
		loadAddon = vi.fn();
		buffer = { active: { baseY: 0, cursorY: 0 } };
		rows = 24;
	}
	return { Terminal: MockTerminal };
});

vi.mock('@xterm/addon-fit', () => {
	class MockFitAddon {
		fit = vi.fn();
		dispose = vi.fn();
	}
	return { FitAddon: MockFitAddon };
});

vi.mock('../logTailClient', () => {
	return {
		createLogTailClient: vi.fn().mockReturnValue({
			connect: vi.fn(),
			disconnect: vi.fn(),
			sendFilter: vi.fn(),
		}),
	};
});

vi.mock('../logHighlight', () => {
	return {
		highlightMatches: vi.fn((line: string) => line),
	};
});

describe('LogTailPanel', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('renders namespace/pod selector', () => {
		render(LogTailPanel, {
			props: {
				availableServices: [
					{ namespace: 'default', name: 'web-app' },
					{ namespace: 'monitoring', name: 'prometheus' },
				],
			},
		});

		const namespaceSelect = screen.getByLabelText('Namespace');
		expect(namespaceSelect).toBeInTheDocument();

		const podSelect = screen.getByLabelText('Pod');
		expect(podSelect).toBeInTheDocument();
	});

	it('renders terminal container', () => {
		render(LogTailPanel, {
			props: {
				availableServices: [],
			},
		});

		const container = screen.getByTestId('log-terminal');
		expect(container).toBeInTheDocument();
	});

	it('shows connection status', () => {
		render(LogTailPanel, {
			props: {
				availableServices: [{ namespace: 'default', name: 'web-app' }],
			},
		});

		expect(screen.getByText(/select a pod/i)).toBeInTheDocument();
	});

	it('renders filter input', () => {
		render(LogTailPanel, {
			props: {
				availableServices: [],
			},
		});

		const filterInput = screen.getByLabelText('Filter pattern');
		expect(filterInput).toBeInTheDocument();
		expect(filterInput).toHaveAttribute('placeholder', 'Filter logs (regex or text)...');
	});
});
