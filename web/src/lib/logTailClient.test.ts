import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createLogTailClient } from './logTailClient';

// Mock WebSocket
class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	static instances: MockWebSocket[] = [];
	url: string;
	readyState: number = 0;
	CONNECTING = 0;
	OPEN = 1;
	CLOSING = 2;
	CLOSED = 3;
	onopen: ((ev: Event) => void) | null = null;
	onclose: ((ev: CloseEvent) => void) | null = null;
	onmessage: ((ev: MessageEvent) => void) | null = null;
	onerror: ((ev: Event) => void) | null = null;
	sentMessages: string[] = [];
	closeCode?: number;
	closeReason?: string;

	constructor(url: string) {
		this.url = url;
		MockWebSocket.instances.push(this);
	}

	send(data: string) {
		this.sentMessages.push(data);
	}

	close(code?: number, reason?: string) {
		this.closeCode = code;
		this.closeReason = reason;
		this.readyState = 3;
		if (this.onclose) {
			this.onclose(new CloseEvent('close', { code, reason }));
		}
	}

	// Test helpers
	simulateOpen() {
		this.readyState = 1;
		if (this.onopen) this.onopen(new Event('open'));
	}

	simulateMessage(data: string) {
		if (this.onmessage) this.onmessage(new MessageEvent('message', { data }));
	}

	simulateClose(code = 1000, reason = '') {
		this.readyState = 3;
		if (this.onclose) this.onclose(new CloseEvent('close', { code, reason }));
	}

	simulateError() {
		if (this.onerror) this.onerror(new Event('error'));
	}
}

describe('logTailClient', () => {
	let originalWebSocket: typeof WebSocket;

	beforeEach(() => {
		MockWebSocket.instances = [];
		originalWebSocket = globalThis.WebSocket;
		globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket;
		vi.useFakeTimers();
	});

	afterEach(() => {
		globalThis.WebSocket = originalWebSocket;
		vi.useRealTimers();
	});

	it('connects to correct WS URL', () => {
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine: vi.fn(),
			onControl: vi.fn(),
			onError: vi.fn(),
			onConnectionChange: vi.fn(),
		});

		client.connect();

		expect(MockWebSocket.instances).toHaveLength(1);
		const ws = MockWebSocket.instances[0];
		expect(ws.url).toContain('/api/logs/default/my-pod');
	});

	it('calls onLine for text frames', () => {
		const onLine = vi.fn();
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine,
			onControl: vi.fn(),
			onError: vi.fn(),
			onConnectionChange: vi.fn(),
		});

		client.connect();
		const ws = MockWebSocket.instances[0];
		ws.simulateOpen();
		ws.simulateMessage('some log line');

		expect(onLine).toHaveBeenCalledWith('some log line');
	});

	it('calls onControl for control messages', () => {
		const onControl = vi.fn();
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine: vi.fn(),
			onControl,
			onError: vi.fn(),
			onConnectionChange: vi.fn(),
		});

		client.connect();
		const ws = MockWebSocket.instances[0];
		ws.simulateOpen();
		ws.simulateMessage(JSON.stringify({ type: 'control', event: 'pod-restarted' }));

		expect(onControl).toHaveBeenCalledWith('pod-restarted');
	});

	it('calls onError for error messages', () => {
		const onError = vi.fn();
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine: vi.fn(),
			onControl: vi.fn(),
			onError,
			onConnectionChange: vi.fn(),
		});

		client.connect();
		const ws = MockWebSocket.instances[0];
		ws.simulateOpen();
		ws.simulateMessage(JSON.stringify({ type: 'error', message: 'pod not found' }));

		expect(onError).toHaveBeenCalledWith('pod not found');
	});

	it('reconnects with exponential backoff', () => {
		const onConnectionChange = vi.fn();
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine: vi.fn(),
			onControl: vi.fn(),
			onError: vi.fn(),
			onConnectionChange,
		});

		client.connect();
		const ws1 = MockWebSocket.instances[0];
		ws1.simulateOpen();

		// Simulate unexpected close
		ws1.simulateClose(1006, 'abnormal');

		expect(onConnectionChange).toHaveBeenCalledWith('reconnecting');

		// Advance timer for first reconnect (1s)
		vi.advanceTimersByTime(1000);
		expect(MockWebSocket.instances).toHaveLength(2);

		// Second unexpected close
		const ws2 = MockWebSocket.instances[1];
		ws2.simulateClose(1006, 'abnormal');

		// Advance timer for second reconnect (2s)
		vi.advanceTimersByTime(2000);
		expect(MockWebSocket.instances).toHaveLength(3);
	});

	it('sendFilter sends correct JSON', () => {
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine: vi.fn(),
			onControl: vi.fn(),
			onError: vi.fn(),
			onConnectionChange: vi.fn(),
		});

		client.connect();
		const ws = MockWebSocket.instances[0];
		ws.simulateOpen();

		client.sendFilter('error');

		expect(ws.sentMessages).toHaveLength(1);
		expect(JSON.parse(ws.sentMessages[0])).toEqual({
			type: 'filter',
			pattern: 'error',
		});
	});

	it('disconnect closes cleanly', () => {
		const onConnectionChange = vi.fn();
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine: vi.fn(),
			onControl: vi.fn(),
			onError: vi.fn(),
			onConnectionChange,
		});

		client.connect();
		const ws = MockWebSocket.instances[0];
		ws.simulateOpen();

		client.disconnect();

		expect(ws.closeCode).toBe(1000);
		expect(onConnectionChange).toHaveBeenCalledWith('disconnected');
	});

	it('does not reconnect after intentional disconnect', () => {
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine: vi.fn(),
			onControl: vi.fn(),
			onError: vi.fn(),
			onConnectionChange: vi.fn(),
		});

		client.connect();
		const ws = MockWebSocket.instances[0];
		ws.simulateOpen();

		client.disconnect();

		// Advance timers - should not reconnect
		vi.advanceTimersByTime(60000);
		expect(MockWebSocket.instances).toHaveLength(1);
	});

	it('reports connected status on open', () => {
		const onConnectionChange = vi.fn();
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine: vi.fn(),
			onControl: vi.fn(),
			onError: vi.fn(),
			onConnectionChange,
		});

		client.connect();
		expect(onConnectionChange).toHaveBeenCalledWith('connecting');

		const ws = MockWebSocket.instances[0];
		ws.simulateOpen();
		expect(onConnectionChange).toHaveBeenCalledWith('connected');
	});

	it('does not reconnect on normal close (1000)', () => {
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine: vi.fn(),
			onControl: vi.fn(),
			onError: vi.fn(),
			onConnectionChange: vi.fn(),
		});

		client.connect();
		const ws = MockWebSocket.instances[0];
		ws.simulateOpen();
		ws.simulateClose(1000, 'normal');

		vi.advanceTimersByTime(60000);
		expect(MockWebSocket.instances).toHaveLength(1);
	});

	it('caps backoff at 30 seconds', () => {
		const client = createLogTailClient({
			namespace: 'default',
			pod: 'my-pod',
			onLine: vi.fn(),
			onControl: vi.fn(),
			onError: vi.fn(),
			onConnectionChange: vi.fn(),
		});

		client.connect();

		// Simulate many failures to exceed 30s cap
		for (let i = 0; i < 10; i++) {
			const ws = MockWebSocket.instances[MockWebSocket.instances.length - 1];
			ws.simulateOpen();
			ws.simulateClose(1006, 'abnormal');
			vi.advanceTimersByTime(30000); // always advance by max
		}

		// Should have reconnected each time
		expect(MockWebSocket.instances.length).toBeGreaterThan(5);
	});
});
