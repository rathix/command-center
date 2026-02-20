export function formatRelativeTime(isoTimestamp: string | null): string {
	if (!isoTimestamp) return 'unknown';
	const now = Date.now();
	const then = new Date(isoTimestamp).getTime();
	if (Number.isNaN(then)) return 'unknown';

	const diffMs = now - then;
	const diffSec = Math.max(0, Math.floor(diffMs / 1000));

	if (diffSec < 60) return `${diffSec}s ago`;
	const diffMin = Math.floor(diffSec / 60);
	if (diffMin < 60) return `${diffMin}m ago`;
	const diffHour = Math.floor(diffMin / 60);
	return `${diffHour}h ago`;
}
