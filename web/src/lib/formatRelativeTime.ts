export function formatRelativeTime(
	isoTimestamp: string | null,
	includeSuffix = true,
	precise = false
): string {
	if (!isoTimestamp) return 'unknown';
	const now = Date.now();
	const then = new Date(isoTimestamp).getTime();
	if (Number.isNaN(then)) return 'unknown';

	const diffMs = now - then;
	const diffSec = Math.max(0, Math.floor(diffMs / 1000));

	if (precise) {
		const hours = Math.floor(diffSec / 3600);
		const minutes = Math.floor((diffSec % 3600) / 60);
		const seconds = diffSec % 60;

		if (hours > 0) {
			return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
		}
		if (minutes > 0) {
			return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
		}
		return `${seconds}s`;
	}

	const suffix = includeSuffix ? ' ago' : '';

	if (diffSec < 60) return `${diffSec}s${suffix}`;
	const diffMin = Math.floor(diffSec / 60);
	if (diffMin < 60) return `${diffMin}m${suffix}`;
	const diffHour = Math.floor(diffMin / 60);
	if (diffHour < 24) return `${diffHour}h${suffix}`;
	const diffDay = Math.floor(diffHour / 24);
	if (diffDay < 7) return `${diffDay}d${suffix}`;
	const diffWeek = Math.floor(diffDay / 7);
	return `${diffWeek}w${suffix}`;
}
