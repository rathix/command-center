/**
 * Returns the base path from the <base> tag if present, otherwise '/'.
 * Used to construct API URLs that work with both direct and proxied access.
 */
export function getBasePath(): string {
	if (typeof document === 'undefined') return '/';
	const baseTag = document.querySelector('base');
	if (baseTag) {
		const href = baseTag.getAttribute('href');
		if (href) return href;
	}
	return '/';
}

/**
 * Constructs an absolute URL relative to the current base path.
 * @param path - The path to resolve (e.g., 'api/events')
 * @returns The full URL string
 */
export function resolveApiUrl(path: string): string {
	const base = getBasePath();
	// Remove leading slash from path if base ends with slash
	const cleanPath = path.startsWith('/') ? path.slice(1) : path;
	const cleanBase = base.endsWith('/') ? base : base + '/';
	return cleanBase + cleanPath;
}
