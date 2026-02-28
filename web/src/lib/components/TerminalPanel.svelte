<script lang="ts">
	import { Terminal } from 'xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import { WebLinksAddon } from '@xterm/addon-web-links';
	import { TerminalClient } from '../terminalClient';
	import type { WSClientState } from '../wsClient';

	interface Props {
		command?: string;
		wsUrl?: string;
		sessionId?: string;
		onClose?: () => void;
	}

	let { command = 'kubectl', wsUrl = '/api/terminal', sessionId = '', onClose }: Props = $props();

	let containerRef: HTMLDivElement | undefined = $state(undefined);
	let connectionState = $state<WSClientState>('disconnected');
	let terminatedReason: string | null = $state(null);
	let errorMessage: string | null = $state(null);

	const isDisconnected = $derived(connectionState === 'disconnected' && !terminatedReason);
	const isReconnecting = $derived(connectionState === 'reconnecting');
	const isTerminated = $derived(terminatedReason !== null);

	const showOverlay = $derived(isDisconnected || isReconnecting || isTerminated || errorMessage !== null);

	$effect(() => {
		if (!containerRef) return;

		const term = new Terminal({
			cursorBlink: true,
			fontSize: 14,
			fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace',
			theme: {
				background: '#0f172a',
				foreground: '#e2e8f0',
				cursor: '#38bdf8',
				selectionBackground: '#334155',
				black: '#0f172a',
				red: '#ef4444',
				green: '#22c55e',
				yellow: '#eab308',
				blue: '#3b82f6',
				magenta: '#a855f7',
				cyan: '#06b6d4',
				white: '#e2e8f0',
				brightBlack: '#475569',
				brightRed: '#f87171',
				brightGreen: '#4ade80',
				brightYellow: '#facc15',
				brightBlue: '#60a5fa',
				brightMagenta: '#c084fc',
				brightCyan: '#22d3ee',
				brightWhite: '#f8fafc'
			}
		});

		const fitAddon = new FitAddon();
		const webLinksAddon = new WebLinksAddon();
		term.loadAddon(fitAddon);
		term.loadAddon(webLinksAddon);
		term.open(containerRef);

		// Initial fit
		try {
			fitAddon.fit();
		} catch {
			// May fail if container has no size yet
		}

		// Create terminal client
		const url = `${wsUrl}?command=${encodeURIComponent(command)}`;
		const client = new TerminalClient(url, {
			onData: (data: Uint8Array) => {
				term.write(data);
			},
			onError: (message: string) => {
				errorMessage = message;
				term.write(`\r\n\x1b[31mError: ${message}\x1b[0m\r\n`);
			},
			onTerminated: (reason: string) => {
				terminatedReason = reason;
				term.write(`\r\n\x1b[33mSession terminated: ${reason}\x1b[0m\r\n`);
			},
			onStateChange: (state: WSClientState) => {
				connectionState = state;
			},
			onReconnecting: (attempt: number) => {
				term.write(`\r\n\x1b[33mReconnecting (attempt ${attempt + 1})...\x1b[0m\r\n`);
			}
		});

		// Wire terminal input to client
		const inputDisposable = term.onData((data: string) => {
			client.send(data);
		});

		// Connect
		client.connect();

		// Resize handling with debounce
		let resizeTimer: ReturnType<typeof setTimeout> | undefined;
		const resizeObserver = new ResizeObserver(() => {
			clearTimeout(resizeTimer);
			resizeTimer = setTimeout(() => {
				try {
					fitAddon.fit();
					client.sendResize(term.cols, term.rows);
				} catch {
					// Ignore resize errors during cleanup
				}
			}, 100);
		});
		resizeObserver.observe(containerRef);

		return () => {
			clearTimeout(resizeTimer);
			resizeObserver.disconnect();
			inputDisposable.dispose();
			client.close();
			term.dispose();
		};
	});
</script>

<div class="relative h-full w-full" data-testid="terminal-panel" data-session-id={sessionId}>
	<div bind:this={containerRef} class="h-full w-full bg-slate-900"></div>

	{#if showOverlay}
		<div
			class="absolute inset-0 flex items-center justify-center bg-slate-900/80"
			data-testid="terminal-overlay"
		>
			<div class="text-center">
				{#if isReconnecting}
					<div class="mb-2 h-6 w-6 mx-auto animate-spin rounded-full border-2 border-sky-400 border-t-transparent"></div>
					<p class="text-slate-300">Disconnected — reconnecting...</p>
				{:else if isTerminated}
					<p class="text-amber-400 mb-2">Session terminated — {terminatedReason}</p>
					{#if onClose}
						<button
							class="rounded bg-slate-700 px-3 py-1 text-sm text-slate-300 hover:bg-slate-600"
							onclick={onClose}
						>
							Close
						</button>
					{/if}
				{:else if errorMessage}
					<p class="text-red-400 mb-2">{errorMessage}</p>
					{#if onClose}
						<button
							class="rounded bg-slate-700 px-3 py-1 text-sm text-slate-300 hover:bg-slate-600"
							onclick={onClose}
						>
							Close
						</button>
					{/if}
				{:else if isDisconnected}
					<p class="text-slate-400">Disconnected</p>
				{/if}
			</div>
		</div>
	{/if}
</div>
