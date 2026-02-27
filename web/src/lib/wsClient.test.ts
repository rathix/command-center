import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { WSClient, calculateBackoff } from './wsClient';

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
	closeCalled = false;
	closeCode = 0;
	closeReason = '';
	sentMessages: (string | Uint8Array)[] = [];

	constructor(url: string) {
		this.url = url;
		MockWebSocket.instances.push(this);
	}

	send(data: string | Uint8Array) {
		this.sentMessages.push(data);
	}

	close(code?: number, reason?: string) {
		this.closeCalled = true;
		this.closeCode = code ?? 1000;
		this.closeReason = reason ?? '';
		this.readyState = MockWebSocket.CLOSED;
	}

	// Helpers for tests
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

describe('calculateBackoff', () => {
	it('produces correct exponential sequence without jitter', () => {
		const delays = [0, 1, 2, 3, 4, 5, 6].map((attempt) =>
			calculateBackoff(attempt, 1000, 2, 30000, false)
		);
		expect(delays).toEqual([1000, 2000, 4000, 8000, 16000, 30000, 30000]);
	});

	it('caps at maxDelay', () => {
		const delay = calculateBackoff(100, 1000, 2, 30000, false);
		expect(delay).toBe(30000);
	});

	it('applies jitter within expected range', () => {
		vi.spyOn(Math, 'random').mockReturnValue(0.5);
		const delay = calculateBackoff(0, 1000, 2, 30000, true);
		// 1000 * (0.5 + 0.5 * 0.5) = 1000 * 0.75 = 750
		expect(delay).toBe(750);
		vi.restoreAllMocks();
	});
});

describe('WSClient', () => {
	describe('connect', () => {
		it('creates WebSocket with correct URL', () => {
			const client = new WSClient('ws://localhost:8443/api/terminal', {});
			client.connect();
			expect(MockWebSocket.instances).toHaveLength(1);
			expect(MockWebSocket.instances[0].url).toBe('ws://localhost:8443/api/terminal');
		});

		it('sets binaryType to arraybuffer', () => {
			const client = new WSClient('ws://localhost/test', {});
			client.connect();
			expect(MockWebSocket.instances[0].binaryType).toBe('arraybuffer');
		});

		it('calls onOpen callback when connection opens', () => {
			const onOpen = vi.fn();
			const client = new WSClient('ws://localhost/test', { onOpen });
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			expect(onOpen).toHaveBeenCalledOnce();
		});

		it('transitions state to connected on open', () => {
			const onStateChange = vi.fn();
			const client = new WSClient('ws://localhost/test', { onStateChange });
			client.connect();
			expect(onStateChange).toHaveBeenCalledWith('connecting');
			MockWebSocket.instances[0].simulateOpen();
			expect(onStateChange).toHaveBeenCalledWith('connected');
		});
	});

	describe('send', () => {
		it('sends binary data when connected', () => {
			const client = new WSClient('ws://localhost/test', {});
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			const data = new Uint8Array([1, 2, 3]);
			client.sendBinary(data);
			expect(MockWebSocket.instances[0].sentMessages).toHaveLength(1);
		});

		it('sends text data when connected', () => {
			const client = new WSClient('ws://localhost/test', {});
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			client.sendText('hello');
			expect(MockWebSocket.instances[0].sentMessages[0]).toBe('hello');
		});

		it('does not send when not connected', () => {
			const client = new WSClient('ws://localhost/test', {});
			client.connect();
			// Don't simulate open
			client.sendBinary(new Uint8Array([1]));
			client.sendText('hello');
			expect(MockWebSocket.instances[0].sentMessages).toHaveLength(0);
		});
	});

	describe('messages', () => {
		it('calls onMessage with received data', () => {
			const onMessage = vi.fn();
			const client = new WSClient('ws://localhost/test', { onMessage });
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			const data = new ArrayBuffer(3);
			MockWebSocket.instances[0].simulateMessage(data);
			expect(onMessage).toHaveBeenCalledWith(data);
		});
	});

	describe('reconnection', () => {
		it('reconnects with exponential backoff on unexpected close', () => {
			const onReconnecting = vi.fn();
			const client = new WSClient(
				'ws://localhost/test',
				{ onReconnecting },
				{ initialDelay: 1000, multiplier: 2, maxDelay: 30000, jitter: false }
			);
			client.connect();
			const ws1 = MockWebSocket.instances[0];
			ws1.simulateOpen();

			// Simulate unexpected close
			ws1.simulateClose(1006, 'abnormal');
			expect(onReconnecting).toHaveBeenCalledWith(0, 1000);

			// Advance timer to trigger reconnect
			vi.advanceTimersByTime(1000);
			expect(MockWebSocket.instances).toHaveLength(2);

			// Second close triggers backoff with 2s
			MockWebSocket.instances[1].simulateClose(1006, 'abnormal');
			expect(onReconnecting).toHaveBeenCalledWith(1, 2000);
		});

		it('does not reconnect on manual close', () => {
			const onReconnecting = vi.fn();
			const client = new WSClient('ws://localhost/test', { onReconnecting });
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			client.close();
			expect(onReconnecting).not.toHaveBeenCalled();
		});

		it('does not reconnect when autoReconnect is false', () => {
			const onReconnecting = vi.fn();
			const client = new WSClient(
				'ws://localhost/test',
				{ onReconnecting },
				{ autoReconnect: false }
			);
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			MockWebSocket.instances[0].simulateClose(1006, 'abnormal');
			expect(onReconnecting).not.toHaveBeenCalled();
		});

		it('stops reconnect when stopReconnect is called', () => {
			const onReconnecting = vi.fn();
			const client = new WSClient(
				'ws://localhost/test',
				{ onReconnecting },
				{ initialDelay: 1000, jitter: false }
			);
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			MockWebSocket.instances[0].simulateClose(1006, 'abnormal');

			// Reconnecting was scheduled
			expect(onReconnecting).toHaveBeenCalledOnce();

			// Stop reconnect before timer fires
			client.stopReconnect();

			// Advance past the reconnect delay
			vi.advanceTimersByTime(5000);

			// No new connection was made
			expect(MockWebSocket.instances).toHaveLength(1);
		});

		it('resets reconnect attempt counter on successful connection', () => {
			const onReconnecting = vi.fn();
			const client = new WSClient(
				'ws://localhost/test',
				{ onReconnecting },
				{ initialDelay: 1000, multiplier: 2, maxDelay: 30000, jitter: false }
			);
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			MockWebSocket.instances[0].simulateClose(1006);
			expect(onReconnecting).toHaveBeenCalledWith(0, 1000);

			vi.advanceTimersByTime(1000);
			// Successfully reconnected
			MockWebSocket.instances[1].simulateOpen();

			// Close again — should start from attempt 0
			MockWebSocket.instances[1].simulateClose(1006);
			expect(onReconnecting).toHaveBeenCalledWith(0, 1000);
		});
	});

	describe('close', () => {
		it('closes the WebSocket with code 1000', () => {
			const client = new WSClient('ws://localhost/test', {});
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			client.close();
			expect(MockWebSocket.instances[0].closeCalled).toBe(true);
			expect(MockWebSocket.instances[0].closeCode).toBe(1000);
		});

		it('transitions state to disconnected', () => {
			const onStateChange = vi.fn();
			const client = new WSClient('ws://localhost/test', { onStateChange });
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			client.close();
			expect(client.getState()).toBe('disconnected');
		});

		it('calls onClose callback', () => {
			const onClose = vi.fn();
			const client = new WSClient('ws://localhost/test', { onClose });
			client.connect();
			MockWebSocket.instances[0].simulateOpen();
			MockWebSocket.instances[0].simulateClose(1000, 'normal');
			expect(onClose).toHaveBeenCalledWith(1000, 'normal');
		});
	});
});
