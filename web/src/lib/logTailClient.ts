export type LogConnectionStatus = 'connected' | 'connecting' | 'reconnecting' | 'disconnected';

export interface LogTailClientOptions {
	namespace: string;
	pod: string;
	onLine: (line: string) => void;
	onControl: (event: string) => void;
	onError: (message: string) => void;
	onConnectionChange: (status: LogConnectionStatus) => void;
}

export interface LogTailClient {
	connect: () => void;
	disconnect: () => void;
	sendFilter: (pattern: string) => void;
}

interface ControlMessage {
	type: 'control';
	event: string;
}

interface ErrorMessage {
	type: 'error';
	message: string;
}

function isControlMessage(data: unknown): data is ControlMessage {
	return (
		typeof data === 'object' &&
		data !== null &&
		(data as Record<string, unknown>).type === 'control' &&
		typeof (data as Record<string, unknown>).event === 'string'
	);
}

function isErrorMessage(data: unknown): data is ErrorMessage {
	return (
		typeof data === 'object' &&
		data !== null &&
		(data as Record<string, unknown>).type === 'error' &&
		typeof (data as Record<string, unknown>).message === 'string'
	);
}

export function createLogTailClient(options: LogTailClientOptions): LogTailClient {
	let ws: WebSocket | null = null;
	let intentionalClose = false;
	let reconnectAttempt = 0;
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

	const MAX_BACKOFF_MS = 30_000;
	const BASE_BACKOFF_MS = 1_000;

	function getBackoffMs(): number {
		const backoff = Math.min(BASE_BACKOFF_MS * Math.pow(2, reconnectAttempt), MAX_BACKOFF_MS);
		return backoff;
	}

	function buildUrl(): string {
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		return `${protocol}//${window.location.host}/api/logs/${options.namespace}/${options.pod}`;
	}

	function handleMessage(event: MessageEvent) {
		const data = event.data as string;

		// Try to parse as JSON first (control/error messages)
		try {
			const parsed: unknown = JSON.parse(data);
			if (isControlMessage(parsed)) {
				options.onControl(parsed.event);
				return;
			}
			if (isErrorMessage(parsed)) {
				options.onError(parsed.message);
				return;
			}
		} catch {
			// Not JSON, treat as log line
		}

		// Plain text log line
		options.onLine(data);
	}

	function scheduleReconnect() {
		if (intentionalClose) return;

		options.onConnectionChange('reconnecting');
		const delay = getBackoffMs();
		reconnectAttempt++;

		reconnectTimer = setTimeout(() => {
			reconnectTimer = null;
			connect();
		}, delay);
	}

	function connect() {
		intentionalClose = false;
		options.onConnectionChange('connecting');

		const socket = new WebSocket(buildUrl());
		ws = socket;

		socket.onopen = () => {
			reconnectAttempt = 0;
			options.onConnectionChange('connected');
		};

		socket.onmessage = handleMessage;

		socket.onclose = (event: CloseEvent) => {
			ws = null;
			if (intentionalClose || event.code === 1000) {
				options.onConnectionChange('disconnected');
				return;
			}
			scheduleReconnect();
		};

		socket.onerror = () => {
			// onclose will fire after onerror, so reconnect is handled there
		};
	}

	function disconnect() {
		intentionalClose = true;
		if (reconnectTimer !== null) {
			clearTimeout(reconnectTimer);
			reconnectTimer = null;
		}
		if (ws) {
			ws.close(1000, 'client disconnect');
		}
		options.onConnectionChange('disconnected');
	}

	function sendFilter(pattern: string) {
		if (ws && ws.readyState === WebSocket.OPEN) {
			ws.send(JSON.stringify({ type: 'filter', pattern }));
		}
	}

	return { connect, disconnect, sendFilter };
}
