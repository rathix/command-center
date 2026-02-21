import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { formatRelativeTime } from './formatRelativeTime';

describe('formatRelativeTime', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-02-20T12:00:00Z'));
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('returns "unknown" for null input', () => {
		expect(formatRelativeTime(null)).toBe('unknown');
	});

	it('returns "unknown" for invalid timestamp input', () => {
		expect(formatRelativeTime('not-a-date')).toBe('unknown');
	});

	it('returns seconds format for less than 60 seconds', () => {
		expect(formatRelativeTime('2026-02-20T11:59:30Z')).toBe('30s ago');
	});

	it('returns "0s ago" for exactly now', () => {
		expect(formatRelativeTime('2026-02-20T12:00:00Z')).toBe('0s ago');
	});

	it('clamps future timestamps to "0s ago"', () => {
		expect(formatRelativeTime('2026-02-20T12:00:05Z')).toBe('0s ago');
	});

	it('returns "59s ago" at the boundary before minutes', () => {
		expect(formatRelativeTime('2026-02-20T11:59:01Z')).toBe('59s ago');
	});

	it('returns minutes format for 1-59 minutes', () => {
		expect(formatRelativeTime('2026-02-20T11:55:00Z')).toBe('5m ago');
	});

	it('returns "1m ago" at exactly 60 seconds', () => {
		expect(formatRelativeTime('2026-02-20T11:59:00Z')).toBe('1m ago');
	});

	it('returns "59m ago" at the boundary before hours', () => {
		expect(formatRelativeTime('2026-02-20T11:01:00Z')).toBe('59m ago');
	});

	it('returns hours format for 1 hour or more', () => {
		expect(formatRelativeTime('2026-02-20T10:00:00Z')).toBe('2h ago');
	});

	it('returns "1h ago" at exactly 60 minutes', () => {
		expect(formatRelativeTime('2026-02-20T11:00:00Z')).toBe('1h ago');
	});

	it('returns "23h ago" at the boundary before days', () => {
		expect(formatRelativeTime('2026-02-19T13:00:00Z')).toBe('23h ago');
	});

	it('returns days format for 1-6 days', () => {
		expect(formatRelativeTime('2026-02-19T12:00:00Z')).toBe('1d ago');
	});

	it('returns "2d ago" for two days', () => {
		expect(formatRelativeTime('2026-02-18T12:00:00Z')).toBe('2d ago');
	});

	it('returns "6d ago" at the boundary before weeks', () => {
		expect(formatRelativeTime('2026-02-14T12:00:00Z')).toBe('6d ago');
	});

	it('returns weeks format for 7+ days', () => {
		expect(formatRelativeTime('2026-02-13T12:00:00Z')).toBe('1w ago');
	});

	it('returns "2w ago" for two weeks', () => {
		expect(formatRelativeTime('2026-02-06T12:00:00Z')).toBe('2w ago');
	});

	describe('includeSuffix: false', () => {
		it('omits suffix for seconds', () => {
			expect(formatRelativeTime('2026-02-20T11:59:30Z', false)).toBe('30s');
		});

		it('omits suffix for minutes', () => {
			expect(formatRelativeTime('2026-02-20T11:55:00Z', false)).toBe('5m');
		});

		it('omits suffix for hours', () => {
			expect(formatRelativeTime('2026-02-20T10:00:00Z', false)).toBe('2h');
		});

		it('omits suffix for days', () => {
			expect(formatRelativeTime('2026-02-18T12:00:00Z', false)).toBe('2d');
		});

		it('omits suffix for weeks', () => {
			expect(formatRelativeTime('2026-02-06T12:00:00Z', false)).toBe('2w');
		});
	});

	describe('precise: true', () => {
		it('returns seconds precision', () => {
			expect(formatRelativeTime('2026-02-20T11:59:45Z', false, true)).toBe('15s');
		});

		it('returns minutes and seconds precision', () => {
			expect(formatRelativeTime('2026-02-20T11:55:30Z', false, true)).toBe('4m 30s');
		});

		it('returns hours and minutes precision', () => {
			expect(formatRelativeTime('2026-02-20T09:30:00Z', false, true)).toBe('2h 30m');
		});

		it('omits seconds when 0 in minutes precision', () => {
			expect(formatRelativeTime('2026-02-20T11:55:00Z', false, true)).toBe('5m');
		});

		it('omits minutes when 0 in hours precision', () => {
			expect(formatRelativeTime('2026-02-20T10:00:00Z', false, true)).toBe('2h');
		});
	});
});
