import { describe, it, expect } from 'vitest';
import { highlightMatches, HIGHLIGHT_START, HIGHLIGHT_END } from './logHighlight';

describe('highlightMatches', () => {
	it('returns unmodified line when pattern is empty', () => {
		expect(highlightMatches('some log line', '')).toBe('some log line');
	});

	it('highlights substring matches with ANSI codes', () => {
		const result = highlightMatches('this has error in it', 'error');
		expect(result).toBe(`this has ${HIGHLIGHT_START}error${HIGHLIGHT_END} in it`);
	});

	it('highlights regex matches', () => {
		const result = highlightMatches('ERROR: something failed', '^ERROR');
		expect(result).toBe(`${HIGHLIGHT_START}ERROR${HIGHLIGHT_END}: something failed`);
	});

	it('falls back to substring on invalid regex', () => {
		const result = highlightMatches('has [invalid bracket', '[invalid');
		expect(result).toBe(`has ${HIGHLIGHT_START}[invalid${HIGHLIGHT_END} bracket`);
	});

	it('handles multiple matches per line', () => {
		const result = highlightMatches('error and another error', 'error');
		expect(result).toBe(
			`${HIGHLIGHT_START}error${HIGHLIGHT_END} and another ${HIGHLIGHT_START}error${HIGHLIGHT_END}`
		);
	});

	it('case-insensitive matching', () => {
		const result = highlightMatches('ERROR and error', 'error');
		expect(result).toBe(
			`${HIGHLIGHT_START}ERROR${HIGHLIGHT_END} and ${HIGHLIGHT_START}error${HIGHLIGHT_END}`
		);
	});

	it('escapes special regex characters for substring fallback', () => {
		const result = highlightMatches('value is 3.14', '3.14');
		// Should match literally "3.14", not "3" + any char + "14"
		expect(result).toBe(`value is ${HIGHLIGHT_START}3.14${HIGHLIGHT_END}`);
	});

	it('regex pattern with groups', () => {
		const result = highlightMatches('2024-01-15 ERROR something', '\\d{4}-\\d{2}-\\d{2}');
		expect(result).toBe(
			`${HIGHLIGHT_START}2024-01-15${HIGHLIGHT_END} ERROR something`
		);
	});

	it('handles empty line', () => {
		expect(highlightMatches('', 'error')).toBe('');
	});

	it('returns unmodified when no match found', () => {
		expect(highlightMatches('all is well', 'error')).toBe('all is well');
	});

	it('handles pattern that is only special regex chars', () => {
		// "()" is invalid as a meaningful regex but valid syntax
		// "*" alone is invalid regex
		const result = highlightMatches('a * b', '*');
		// Should fall back to substring since bare * is invalid regex
		expect(result).toBe(`a ${HIGHLIGHT_START}*${HIGHLIGHT_END} b`);
	});
});
