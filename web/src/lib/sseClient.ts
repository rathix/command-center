import { replaceAll, addOrUpdate, remove, setConnectionStatus } from './serviceStore.svelte';
import type { Service } from './types';

let eventSource: EventSource | null = null;

export function connect(): void {
	setConnectionStatus('connecting');
	eventSource = new EventSource('/api/events');

	eventSource.onopen = () => setConnectionStatus('connected');
	eventSource.onerror = () => setConnectionStatus('disconnected');

	eventSource.addEventListener('state', (e: MessageEvent) => {
		const payload = JSON.parse(e.data) as { services: Service[] };
		replaceAll(payload.services);
	});

	eventSource.addEventListener('discovered', (e: MessageEvent) => {
		const service = JSON.parse(e.data) as Service;
		addOrUpdate(service);
	});

	eventSource.addEventListener('update', (e: MessageEvent) => {
		const service = JSON.parse(e.data) as Service;
		addOrUpdate(service);
	});

	eventSource.addEventListener('removed', (e: MessageEvent) => {
		const payload = JSON.parse(e.data) as { name: string; namespace: string };
		remove(payload.namespace, payload.name);
	});
}

export function disconnect(): void {
	eventSource?.close();
	eventSource = null;
}
