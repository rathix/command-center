/**
 * Generic WebSocket client with exponential backoff reconnection.
 * Reused by terminal (Epic 12) and log streaming (Epic 13).
 */

export type WSClientState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected';

export interface WSClientCallbacks {
	onOpen?: () => void;
	onMessage?: (data: ArrayBuffer | string) => void;
	onClose?: (code: number, reason: string) => void;
	onReconnecting?: (attempt: number, delayMs: number) => void;
	onStateChange?: (state: WSClientState) => void;
}

export interface WSClientOptions {
	/** Initial backoff delay in ms (default 1000) */
	initialDelay?: number;
	/** Backoff multiplier (default 2) */
	multiplier?: number;
	/** Maximum backoff delay in ms (default 30000) */
	maxDelay?: number;
	/** Whether to add jitter to backoff (default true) */
	jitter?: boolean;
	/** Whether to auto-reconnect on disconnect (default true) */
	autoReconnect?: boolean;
}

const DEFAULT_OPTIONS: Required<WSClientOptions> = {
	initialDelay: 1000,
	multiplier: 2,
	maxDelay: 30000,
	jitter: true,
	autoReconnect: true
};

/**
 * Calculates exponential backoff delay with optional jitter.
 * Exported for testing.
 */
export function calculateBackoff(
	attempt: number,
	initialDelay: number,
	multiplier: number,
	maxDelay: number,
	jitter: boolean
): number {
	const delay = Math.min(initialDelay * Math.pow(multiplier, attempt), maxDelay);
	if (!jitter) return delay;
	// Add random jitter: between 0.5x and 1.0x of the delay
	return Math.floor(delay * (0.5 + Math.random() * 0.5));
}

export class WSClient {
	private url: string;
	private ws: WebSocket | null = null;
	private callbacks: WSClientCallbacks;
	private options: Required<WSClientOptions>;
	private reconnectAttempt = 0;
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	private state: WSClientState = 'disconnected';
	private manualClose = false;
	private permanentClose = false;

	constructor(url: string, callbacks: WSClientCallbacks, options?: WSClientOptions) {
		this.url = url;
		this.callbacks = callbacks;
		this.options = { ...DEFAULT_OPTIONS, ...options };
	}

	/** Connect to the WebSocket server. */
	connect(): void {
		this.manualClose = false;
		this.permanentClose = false;
		this.reconnectAttempt = 0;
		this.setState('connecting');
		this.createConnection();
	}

	/** Send a binary message. */
	sendBinary(data: Uint8Array): void {
		if (this.ws?.readyState === WebSocket.OPEN) {
			this.ws.send(data);
		}
	}

	/** Send a text message. */
	sendText(data: string): void {
		if (this.ws?.readyState === WebSocket.OPEN) {
			this.ws.send(data);
		}
	}

	/** Close the connection permanently (no reconnect). */
	close(): void {
		this.manualClose = true;
		this.permanentClose = true;
		this.clearReconnectTimer();
		if (this.ws) {
			this.ws.close(1000, 'client close');
			this.ws = null;
		}
		this.setState('disconnected');
	}

	/**
	 * Stop auto-reconnect without closing the connection.
	 * Used when the server indicates a permanent error (e.g. session limit).
	 */
	stopReconnect(): void {
		this.permanentClose = true;
		this.clearReconnectTimer();
	}

	getState(): WSClientState {
		return this.state;
	}

	private createConnection(): void {
		try {
			this.ws = new WebSocket(this.url);
			this.ws.binaryType = 'arraybuffer';
		} catch {
			this.scheduleReconnect();
			return;
		}

		this.ws.onopen = () => {
			this.reconnectAttempt = 0;
			this.setState('connected');
			this.callbacks.onOpen?.();
		};

		this.ws.onmessage = (event: MessageEvent) => {
			this.callbacks.onMessage?.(event.data);
		};

		this.ws.onclose = (event: CloseEvent) => {
			this.callbacks.onClose?.(event.code, event.reason);

			if (!this.manualClose && !this.permanentClose && this.options.autoReconnect) {
				this.scheduleReconnect();
			} else {
				this.setState('disconnected');
			}
		};

		this.ws.onerror = () => {
			// onerror is always followed by onclose, so we don't need to do much here
		};
	}

	private scheduleReconnect(): void {
		if (this.permanentClose) {
			this.setState('disconnected');
			return;
		}

		const delay = calculateBackoff(
			this.reconnectAttempt,
			this.options.initialDelay,
			this.options.multiplier,
			this.options.maxDelay,
			this.options.jitter
		);

		this.setState('reconnecting');
		this.callbacks.onReconnecting?.(this.reconnectAttempt, delay);
		this.reconnectAttempt++;

		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = null;
			this.createConnection();
		}, delay);
	}

	private clearReconnectTimer(): void {
		if (this.reconnectTimer !== null) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
	}

	private setState(newState: WSClientState): void {
		if (this.state !== newState) {
			this.state = newState;
			this.callbacks.onStateChange?.(newState);
		}
	}
}
