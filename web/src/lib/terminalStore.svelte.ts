/**
 * Terminal session state management using Svelte 5 runes.
 * Follows the getter function pattern from serviceStore.svelte.ts.
 */

import type { WSClientState } from './wsClient';

export interface TerminalSession {
	id: string;
	command: string;
	state: WSClientState;
	createdAt: Date;
	terminatedReason?: string;
}

// Internal reactive state
let sessions = $state(new Map<string, TerminalSession>());
let activeSessionId = $state<string | null>(null);
let nextId = $state(0);

// Derived state
const activeSession = $derived.by(() => {
	if (!activeSessionId) return null;
	return sessions.get(activeSessionId) ?? null;
});

const sessionCount = $derived(sessions.size);

// Exported getter functions (Svelte 5 rune reactivity pattern)
export function getSessions(): Map<string, TerminalSession> {
	return sessions;
}

export function getActiveSession(): TerminalSession | null {
	return activeSession;
}

export function getActiveSessionId(): string | null {
	return activeSessionId;
}

export function getSessionCount(): number {
	return sessionCount;
}

// Mutation functions
export function createSession(command: string = 'kubectl'): string {
	nextId++;
	const id = `session-${nextId}`;
	const session: TerminalSession = {
		id,
		command,
		state: 'connecting',
		createdAt: new Date()
	};
	const updated = new Map(sessions);
	updated.set(id, session);
	sessions = updated;
	activeSessionId = id;
	return id;
}

export function closeSession(id: string): void {
	const updated = new Map(sessions);
	updated.delete(id);
	sessions = updated;
	if (activeSessionId === id) {
		// Activate the first remaining session, or null
		const remaining = [...sessions.keys()];
		activeSessionId = remaining.length > 0 ? remaining[0] : null;
	}
}

export function setActive(id: string): void {
	if (sessions.has(id)) {
		activeSessionId = id;
	}
}

export function updateSessionState(id: string, state: WSClientState): void {
	const session = sessions.get(id);
	if (session) {
		const updated = new Map(sessions);
		updated.set(id, { ...session, state });
		sessions = updated;
	}
}

export function markSessionTerminated(id: string, reason: string): void {
	const session = sessions.get(id);
	if (session) {
		const updated = new Map(sessions);
		updated.set(id, { ...session, state: 'disconnected', terminatedReason: reason });
		sessions = updated;
	}
}

// Test helper — resets all state to initial values
export function _resetForTesting(): void {
	sessions = new Map();
	activeSessionId = null;
	nextId = 0;
}
