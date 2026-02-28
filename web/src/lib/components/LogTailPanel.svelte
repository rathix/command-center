<script lang="ts">
	import { Terminal } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import { createLogTailClient, type LogConnectionStatus } from '../logTailClient';
	import { highlightMatches } from '../logHighlight';

	interface ServiceRef {
		namespace: string;
		name: string;
	}

	let { availableServices = [] }: { availableServices: ServiceRef[] } = $props();

	let selectedNamespace = $state('');
	let selectedPod = $state('');
	let connectionStatus = $state<LogConnectionStatus>('disconnected');
	let isAtBottom = $state(true);
	let hasNewLines = $state(false);
	let filterPattern = $state('');

	// Batched rendering state
	let pendingLines: string[] = [];
	let rafId: number | null = null;

	let terminal: Terminal | null = null;
	let fitAddon: FitAddon | null = null;
	let terminalContainer: HTMLDivElement | undefined = $state();
	let client: ReturnType<typeof createLogTailClient> | null = null;

	// Derive unique namespaces
	const namespaces = $derived(
		[...new Set(availableServices.map((s) => s.namespace))].sort()
	);

	// Derive pods for selected namespace
	const pods = $derived(
		availableServices
			.filter((s) => s.namespace === selectedNamespace)
			.map((s) => s.name)
			.sort()
	);

	function initTerminal(container: HTMLDivElement) {
		if (terminal) {
			terminal.dispose();
		}

		terminal = new Terminal({
			disableStdin: true,
			cursorBlink: false,
			scrollback: 10000,
			convertEol: true,
			theme: {
				background: '#1a1b26',
				foreground: '#a9b1d6',
			},
		});

		fitAddon = new FitAddon();
		terminal.loadAddon(fitAddon);
		terminal.open(container);
		fitAddon.fit();

		terminal.onScroll(() => {
			if (!terminal) return;
			const buf = terminal.buffer.active;
			const atBottom = buf.baseY + terminal.rows >= buf.baseY + buf.cursorY + 1;
			if (atBottom) {
				isAtBottom = true;
				hasNewLines = false;
			} else {
				isAtBottom = false;
			}
		});
	}

	function flushLines() {
		if (pendingLines.length === 0) {
			rafId = requestAnimationFrame(flushLines);
			return;
		}

		const batch = pendingLines.join('\r\n') + '\r\n';
		pendingLines = [];

		if (terminal) {
			terminal.write(batch);
			if (isAtBottom) {
				terminal.scrollToBottom();
			} else {
				hasNewLines = true;
			}
		}

		rafId = requestAnimationFrame(flushLines);
	}

	function startBatchedRendering() {
		if (rafId === null) {
			rafId = requestAnimationFrame(flushLines);
		}
	}

	function stopBatchedRendering() {
		if (rafId !== null) {
			cancelAnimationFrame(rafId);
			rafId = null;
		}
	}

	let filterTimeout: ReturnType<typeof setTimeout> | null = null;

	function onFilterInput() {
		if (filterTimeout) clearTimeout(filterTimeout);
		filterTimeout = setTimeout(() => {
			if (client) {
				client.sendFilter(filterPattern);
			}
		}, 300);
	}

	function clearFilter() {
		filterPattern = '';
		if (filterTimeout) clearTimeout(filterTimeout);
		if (client) {
			client.sendFilter('');
		}
	}

	function handleLine(line: string) {
		const highlighted = filterPattern ? highlightMatches(line, filterPattern) : line;
		pendingLines.push(highlighted);
	}

	function handleControl(event: string) {
		if (event === 'pod-restarted') {
			pendingLines.push('\x1b[1;36m--- Pod restarted ---\x1b[0m');
		}
	}

	function handleError(message: string) {
		pendingLines.push(`\x1b[1;31mError: ${message}\x1b[0m`);
	}

	function handleConnectionChange(status: LogConnectionStatus) {
		connectionStatus = status;
	}

	function connectToPod() {
		if (client) {
			client.disconnect();
			client = null;
		}

		if (!selectedNamespace || !selectedPod) return;

		if (terminal) {
			terminal.clear();
		}
		pendingLines = [];

		client = createLogTailClient({
			namespace: selectedNamespace,
			pod: selectedPod,
			onLine: handleLine,
			onControl: handleControl,
			onError: handleError,
			onConnectionChange: handleConnectionChange,
		});

		startBatchedRendering();
		client.connect();
	}

	function scrollToBottom() {
		if (terminal) {
			terminal.scrollToBottom();
			isAtBottom = true;
			hasNewLines = false;
		}
	}

	function onNamespaceChange() {
		selectedPod = '';
	}

	function onPodChange() {
		if (selectedPod) {
			connectToPod();
		}
	}

	$effect(() => {
		if (terminalContainer) {
			initTerminal(terminalContainer);
		}

		return () => {
			stopBatchedRendering();
			if (client) {
				client.disconnect();
			}
			if (terminal) {
				terminal.dispose();
			}
		};
	});
</script>

<div class="flex h-full flex-col bg-[#1a1b26]">
	<div class="flex items-center gap-2 border-b border-gray-700 bg-gray-900 px-3 py-2">
		<label class="text-xs text-gray-400">
			Namespace
			<select
				class="ml-1 rounded border border-gray-600 bg-gray-800 px-2 py-1 text-xs text-gray-200"
				bind:value={selectedNamespace}
				onchange={onNamespaceChange}
			>
				<option value="">--</option>
				{#each namespaces as ns (ns)}
					<option value={ns}>{ns}</option>
				{/each}
			</select>
		</label>

		<label class="text-xs text-gray-400">
			Pod
			<select
				class="ml-1 rounded border border-gray-600 bg-gray-800 px-2 py-1 text-xs text-gray-200"
				bind:value={selectedPod}
				onchange={onPodChange}
				disabled={!selectedNamespace}
			>
				<option value="">--</option>
				{#each pods as p (p)}
					<option value={p}>{p}</option>
				{/each}
			</select>
		</label>

		<div class="flex items-center gap-1">
			<input
				type="text"
				placeholder="Filter logs (regex or text)..."
				class="rounded border border-gray-600 bg-gray-800 px-2 py-1 text-xs text-gray-200 placeholder-gray-500"
				bind:value={filterPattern}
				oninput={onFilterInput}
				aria-label="Filter pattern"
			/>
			{#if filterPattern}
				<button
					class="rounded border border-gray-600 bg-gray-700 px-2 py-1 text-xs text-gray-300 hover:bg-gray-600"
					onclick={clearFilter}
				>
					Clear
				</button>
			{/if}
		</div>

		<div class="ml-auto flex items-center gap-2">
			{#if connectionStatus === 'connected'}
				<span class="text-xs text-green-400">Connected</span>
			{:else if connectionStatus === 'connecting'}
				<span class="text-xs text-yellow-400">Connecting...</span>
			{:else if connectionStatus === 'reconnecting'}
				<span class="text-xs text-yellow-400">Reconnecting...</span>
			{:else}
				<span class="text-xs text-gray-500">
					{selectedPod ? 'Disconnected' : 'Select a pod'}
				</span>
			{/if}
		</div>
	</div>

	<div class="relative flex-1">
		<div data-testid="log-terminal" bind:this={terminalContainer} class="h-full w-full"></div>

		{#if hasNewLines && !isAtBottom}
			<button
				class="absolute bottom-4 left-1/2 -translate-x-1/2 rounded bg-blue-600 px-3 py-1 text-xs text-white shadow-lg hover:bg-blue-500"
				onclick={scrollToBottom}
			>
				New lines below
			</button>
		{/if}
	</div>
</div>
