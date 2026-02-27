import { describe, it, expect, beforeEach } from 'vitest';
import {
	createSession,
	closeSession,
	setActive,
	getActiveSession,
	getActiveSessionId,
	getSessionCount,
	getSessions,
	updateSessionState,
	markSessionTerminated,
	_resetForTesting
} from './terminalStore.svelte';

beforeEach(() => {
	_resetForTesting();
});

describe('terminalStore', () => {
	describe('createSession', () => {
		it('creates a session with the given command', () => {
			const id = createSession('kubectl');
			expect(id).toMatch(/^session-\d+$/);
			expect(getSessionCount()).toBe(1);
			const session = getSessions().get(id);
			expect(session).toBeDefined();
			expect(session!.command).toBe('kubectl');
			expect(session!.state).toBe('connecting');
		});

		it('automatically sets the new session as active', () => {
			const id = createSession('kubectl');
			expect(getActiveSessionId()).toBe(id);
		});

		it('uses default command when none provided', () => {
			const id = createSession();
			const session = getSessions().get(id);
			expect(session!.command).toBe('kubectl');
		});

		it('increments session IDs', () => {
			const id1 = createSession('kubectl');
			const id2 = createSession('helm');
			expect(id1).not.toBe(id2);
			expect(getSessionCount()).toBe(2);
		});
	});

	describe('closeSession', () => {
		it('removes the session', () => {
			const id = createSession('kubectl');
			closeSession(id);
			expect(getSessionCount()).toBe(0);
			expect(getSessions().has(id)).toBe(false);
		});

		it('clears active session if it was the closed one', () => {
			const id = createSession('kubectl');
			closeSession(id);
			expect(getActiveSessionId()).toBeNull();
		});

		it('activates another session when active one is closed', () => {
			const id1 = createSession('kubectl');
			const id2 = createSession('helm');
			// id2 is active (most recently created)
			expect(getActiveSessionId()).toBe(id2);
			closeSession(id2);
			expect(getActiveSessionId()).toBe(id1);
		});
	});

	describe('setActive', () => {
		it('sets the active session', () => {
			const id1 = createSession('kubectl');
			createSession('helm');
			setActive(id1);
			expect(getActiveSessionId()).toBe(id1);
		});

		it('ignores invalid session IDs', () => {
			const id = createSession('kubectl');
			setActive('nonexistent');
			expect(getActiveSessionId()).toBe(id);
		});
	});

	describe('getActiveSession', () => {
		it('returns the active session', () => {
			const id = createSession('kubectl');
			const session = getActiveSession();
			expect(session).toBeDefined();
			expect(session!.id).toBe(id);
		});

		it('returns null when no sessions', () => {
			expect(getActiveSession()).toBeNull();
		});
	});

	describe('updateSessionState', () => {
		it('updates the session state', () => {
			const id = createSession('kubectl');
			updateSessionState(id, 'connected');
			const session = getSessions().get(id);
			expect(session!.state).toBe('connected');
		});
	});

	describe('markSessionTerminated', () => {
		it('marks session as terminated with reason', () => {
			const id = createSession('kubectl');
			markSessionTerminated(id, 'idle timeout');
			const session = getSessions().get(id);
			expect(session!.state).toBe('disconnected');
			expect(session!.terminatedReason).toBe('idle timeout');
		});
	});

	describe('_resetForTesting', () => {
		it('resets all state', () => {
			createSession('kubectl');
			createSession('helm');
			_resetForTesting();
			expect(getSessionCount()).toBe(0);
			expect(getActiveSessionId()).toBeNull();
		});
	});
});
