export type HealthStatus = 'healthy' | 'unhealthy' | 'authBlocked' | 'unknown';

export const HEALTH_STATUSES: HealthStatus[] = [
	'healthy',
	'unhealthy',
	'authBlocked',
	'unknown'
];

export interface Service {
	name: string;
	namespace: string;
	url: string;
	status: HealthStatus;
	httpCode: number | null;
	responseTimeMs: number | null;
	lastChecked: string | null;
	lastStateChange: string | null;
	errorSnippet: string | null;
}
