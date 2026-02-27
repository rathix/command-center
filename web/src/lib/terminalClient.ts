/**
 * Terminal-specific WebSocket client wrapping the generic wsClient.
 * Handles binary I/O, resize messages, and terminal-specific error handling.
 */

import { WSClient, type WSClientState } from './wsClient';

export interface TerminalClientCallbacks {
	/** Called when binary data is received from the server (PTY output). */
	onData?: (data: Uint8Array) => void;
	/** Called when a JSON error message is received from the server. */
	onError?: (message: string) => void;
	/** Called when a terminated message is received. */
	onTerminated?: (reason: string) => void;
	/** Called when the connection state changes. */
	onStateChange?: (state: WSClientState) => void;
	/** Called when reconnecting with attempt number and delay. */
	onReconnecting?: (attempt: number, delayMs: number) => void;
}

interface ServerMessage {
	type: string;
	message?: string;
	reason?: string;
}

const encoder = new TextEncoder();

export class TerminalClient {
	private client: WSClient;
	private callbacks: TerminalClientCallbacks;

	constructor(url: string, callbacks: TerminalClientCallbacks) {
		this.callbacks = callbacks;
		this.client = new WSClient(url, {
			onOpen: undefined,
			onMessage: (data: ArrayBuffer | string) => this.handleMessage(data),
			onClose: (code: number) => {
				// HTTP 429 comes as a failed upgrade, not a close code.
				// WS close code 4029 = custom code for session limit
				if (code === 4029) {
					this.client.stopReconnect();
				}
			},
			onReconnecting: (attempt: number, delayMs: number) => {
				this.callbacks.onReconnecting?.(attempt, delayMs);
			},
			onStateChange: (state: WSClientState) => {
				this.callbacks.onStateChange?.(state);
			}
		});
	}

	/** Connect to the terminal WebSocket endpoint. */
	connect(): void {
		this.client.connect();
	}

	/** Send keystrokes as binary data. */
	send(data: string): void {
		this.client.sendBinary(encoder.encode(data));
	}

	/** Send a resize message to the backend. */
	sendResize(cols: number, rows: number): void {
		this.client.sendText(JSON.stringify({ type: 'resize', cols, rows }));
	}

	/** Close the connection permanently. */
	close(): void {
		this.client.close();
	}

	/** Stop reconnection attempts (e.g. after permanent error). */
	stopReconnect(): void {
		this.client.stopReconnect();
	}

	/** Get the current connection state. */
	getState(): WSClientState {
		return this.client.getState();
	}

	private handleMessage(data: ArrayBuffer | string): void {
		if (data instanceof ArrayBuffer) {
			// Binary frame: PTY output
			this.callbacks.onData?.(new Uint8Array(data));
		} else if (typeof data === 'string') {
			// Text frame: JSON control message
			try {
				const msg = JSON.parse(data) as ServerMessage;
				if (msg.type === 'error') {
					this.callbacks.onError?.(msg.message ?? 'Unknown error');
				} else if (msg.type === 'terminated') {
					this.client.stopReconnect();
					this.callbacks.onTerminated?.(msg.reason ?? 'unknown');
				}
			} catch {
				// Not valid JSON — ignore
			}
		}
	}
}
