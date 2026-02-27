import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { TerminalClient } from './terminalClient';

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
	sentMessages: (string | Uint8Array | ArrayBuffer)[] = [];

	constructor(url: string) {
		this.url = url;
		MockWebSocket.instances.push(this);
	}

	send(data: string | Uint8Array | ArrayBuffer) {
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

const originalWebSocket = globalThis.WebSocket;

beforeEach(() => {
	MockWebSocket.reset();
	globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
	vi.useFakeTimers();
});

afterEach(() => {
	globalThis.WebSocket = originalWebSocket;
	vi.useRealTimers();
});

describe('TerminalClient', () => {
	describe('connect', () => {
		it('creates WebSocket connection', () => {
			const client = new TerminalClient('ws://localhost/api/terminal', {});
			client.connect();
			expect(MockWebSocket.instances).toHaveLength(1);
			expect(MockWebSocket.instances[0].url).toBe('ws://localhost/api/terminal');
		});
	});

	describe('send', () => {
		it('sends keystrokes as binary data', () => {
			const client = new TerminalClient('ws://localhost/api/terminal', {});
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			client.send('ls\n');
			expect(MockWebSocket.instances[0].sentMessages).toHaveLength(1);
			const sent = MockWebSocket.instances[0].sentMessages[0];
			// In jsdom, Uint8Array may come from a different realm
			expect(ArrayBuffer.isView(sent)).toBe(true);
		});
	});

	describe('sendResize', () => {
		it('sends resize as JSON text', () => {
			const client = new TerminalClient('ws://localhost/api/terminal', {});
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			client.sendResize(80, 24);
			const sent = MockWebSocket.instances[0].sentMessages[0];
			expect(typeof sent).toBe('string');
			const parsed = JSON.parse(sent as string);
			expect(parsed).toEqual({ type: 'resize', cols: 80, rows: 24 });
		});
	});

	describe('onData', () => {
		it('calls onData with binary data from server', () => {
			const onData = vi.fn();
			const client = new TerminalClient('ws://localhost/api/terminal', { onData });
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			const data = new ArrayBuffer(3);
			new Uint8Array(data).set([65, 66, 67]);
			MockWebSocket.instances[0].simulateMessage(data);
			expect(onData).toHaveBeenCalledOnce();
			expect(onData.mock.calls[0][0]).toBeInstanceOf(Uint8Array);
			expect(Array.from(onData.mock.calls[0][0])).toEqual([65, 66, 67]);
		});
	});

	describe('onError', () => {
		it('calls onError for JSON error messages', () => {
			const onError = vi.fn();
			const client = new TerminalClient('ws://localhost/api/terminal', { onError });
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			MockWebSocket.instances[0].simulateMessage(
				JSON.stringify({ type: 'error', message: 'Command not allowed' })
			);
			expect(onError).toHaveBeenCalledWith('Command not allowed');
		});
	});

	describe('onTerminated', () => {
		it('calls onTerminated for terminated messages', () => {
			const onTerminated = vi.fn();
			const client = new TerminalClient('ws://localhost/api/terminal', { onTerminated });
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			MockWebSocket.instances[0].simulateMessage(
				JSON.stringify({ type: 'terminated', reason: 'idle timeout' })
			);
			expect(onTerminated).toHaveBeenCalledWith('idle timeout');
		});

		it('stops reconnect after terminated message', () => {
			const onReconnecting = vi.fn();
			const client = new TerminalClient('ws://localhost/api/terminal', { onReconnecting });
			client.connect();
			MockWebSocket.instances[0].simulateOpen();

			// Receive terminated message
			MockWebSocket.instances[0].simulateMessage(
				JSON.stringify({ type: 'terminated', reason: 'idle timeout' })
			);

			// Now disconnect
			MockWebSocket.instances[0].simulateClose(1000, 'normal');

			// Should NOT reconnect
			vi.advanceTimersByTime(5000);
			expect(MockWebSocket.instances).toHaveLength(1);
		});
	});

	describe('state changes', () => {
		it('reports state changes through callback', () => {
			const onStateChange = vi.fn();
			const client = new TerminalClient('ws://localhost/api/terminal', { onStateChange });
			client.connect();
			expect(onStateChange).toHaveBeenCalledWith('connecting');
			MockWebSocket.instances[0].simulateOpen();
			expect(onStateChange).toHaveBeenCalledWith('connected');
		});
	});

	describe('reconnecting', () => {
		it('fires onReconnecting callback on unexpected disconnect', () => {
			const onReconnecting = vi.fn();
			const client = new TerminalClient('ws://localhost/api/terminal', { onReconnecting });
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			MockWebSocket.instances[0].simulateClose(1006, 'abnormal');
			expect(onReconnecting).toHaveBeenCalledOnce();
		});
	});

	describe('close', () => {
		it('closes the connection', () => {
			const client = new TerminalClient('ws://localhost/api/terminal', {});
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			client.close();
			expect(client.getState()).toBe('disconnected');
		});
	});
});
