import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import TerminalPanel from './TerminalPanel.svelte';

// Mock xterm.js — must use function keyword for constructors
const mockDispose = vi.fn();
const mockOpen = vi.fn();
const mockWrite = vi.fn();
const mockLoadAddon = vi.fn();
const mockOnData = vi.fn().mockReturnValue({ dispose: vi.fn() });

vi.mock('xterm', () => {
	return {
		Terminal: function () {
			return {
				open: mockOpen,
				write: mockWrite,
				dispose: mockDispose,
				loadAddon: mockLoadAddon,
				onData: mockOnData,
				cols: 80,
				rows: 24
			};
		}
	};
});

vi.mock('@xterm/addon-fit', () => {
	return {
		FitAddon: function () {
			return { fit: vi.fn() };
		}
	};
});

vi.mock('@xterm/addon-web-links', () => {
	return {
		WebLinksAddon: function () {
			return {};
		}
	};
});

// Mock WebSocket
class MockWebSocket {
	static instances: MockWebSocket[] = [];
	static readonly CONNECTING = 0;
	static readonly OPEN = 1;
	static readonly CLOSING = 2;
	static readonly CLOSED = 3;

	url: string;
	binaryType = '';
	readyState = MockWebSocket.CONNECTING;
	onopen: (() => void) | null = null;
	onclose: ((event: { code: number; reason: string }) => void) | null = null;
	onmessage: ((event: { data: unknown }) => void) | null = null;
	onerror: (() => void) | null = null;
	sentMessages: unknown[] = [];

	constructor(url: string) {
		this.url = url;
		MockWebSocket.instances.push(this);
	}

	send(data: unknown) {
		this.sentMessages.push(data);
	}

	close() {
		this.readyState = MockWebSocket.CLOSED;
	}

	simulateOpen() {
		this.readyState = MockWebSocket.OPEN;
		this.onopen?.();
	}

	simulateClose(code = 1006, reason = '') {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.({ code, reason });
	}

	simulateMessage(data: unknown) {
		this.onmessage?.({ data });
	}

	static reset() {
		MockWebSocket.instances = [];
	}
}

// Mock ResizeObserver
class MockResizeObserver {
	callback: ResizeObserverCallback;
	constructor(callback: ResizeObserverCallback) {
		this.callback = callback;
	}
	observe() {}
	unobserve() {}
	disconnect() {}
}

const originalWebSocket = globalThis.WebSocket;
const originalResizeObserver = globalThis.ResizeObserver;

beforeEach(() => {
	MockWebSocket.reset();
	globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
	globalThis.ResizeObserver = MockResizeObserver as unknown as typeof ResizeObserver;
	vi.clearAllMocks();
});

afterEach(() => {
	globalThis.WebSocket = originalWebSocket;
	globalThis.ResizeObserver = originalResizeObserver;
});

describe('TerminalPanel', () => {
	it('renders the terminal container', () => {
		render(TerminalPanel, { props: { command: 'kubectl' } });
		const panel = screen.getByTestId('terminal-panel');
		expect(panel).toBeTruthy();
	});

	it('creates xterm Terminal instance and opens it', () => {
		render(TerminalPanel, { props: { command: 'kubectl' } });
		expect(mockOpen).toHaveBeenCalled();
	});

	it('creates WebSocket connection with command parameter', () => {
		render(TerminalPanel, { props: { command: 'helm' } });
		expect(MockWebSocket.instances.length).toBeGreaterThan(0);
		const ws = MockWebSocket.instances[0];
		expect(ws.url).toContain('command=helm');
	});

	it('disposes xterm on unmount', () => {
		const { unmount } = render(TerminalPanel, { props: { command: 'kubectl' } });
		unmount();
		expect(mockDispose).toHaveBeenCalled();
	});

	it('shows reconnecting overlay when WS disconnects', async () => {
		render(TerminalPanel, { props: { command: 'kubectl' } });
		const ws = MockWebSocket.instances[0];
		ws.simulateOpen();

		// Simulate disconnect
		ws.simulateClose(1006, 'abnormal');

		// Wait for Svelte to re-render
		await new Promise((resolve) => setTimeout(resolve, 0));

		const overlay = screen.queryByTestId('terminal-overlay');
		expect(overlay).toBeTruthy();
	});
});
