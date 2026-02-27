export const HIGHLIGHT_START = '\x1b[1;33m'; // Bold yellow
export const HIGHLIGHT_END = '\x1b[0m'; // Reset

function escapeRegExp(str: string): string {
	return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Highlights matching portions of a log line with ANSI bold/yellow escape codes.
 * Tries the pattern as a regex first; falls back to case-insensitive substring on invalid regex.
 */
export function highlightMatches(line: string, pattern: string): string {
	if (!pattern || !line) return line;

	let regex: RegExp;
	try {
		regex = new RegExp(pattern, 'gi');
	} catch {
		// Invalid regex, fall back to escaped substring (case-insensitive)
		const escaped = escapeRegExp(pattern);
		regex = new RegExp(escaped, 'gi');
	}

	return line.replace(regex, (match) => `${HIGHLIGHT_START}${match}${HIGHLIGHT_END}`);
}
