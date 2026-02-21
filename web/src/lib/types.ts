export type HealthStatus = 'healthy' | 'unhealthy' | 'authBlocked' | 'unknown';
export type ServiceSource = 'kubernetes' | 'config';

export type ConnectionStatus = 'connected' | 'connecting' | 'reconnecting' | 'disconnected';

export const CONNECTION_STATUSES: ConnectionStatus[] = [
	'connected',
	'connecting',
	'reconnecting',
	'disconnected'
];

export const HEALTH_STATUSES: HealthStatus[] = [
	'healthy',
	'unhealthy',
	'authBlocked',
	'unknown'
];

export const DEFAULT_HEALTH_CHECK_INTERVAL_MS = 30_000;

export interface Service {
	name: string;
	icon?: string | null;
	displayName: string;
	namespace: string;
	group: string;
	url: string;
	source?: ServiceSource;
	status: HealthStatus;
	httpCode: number | null;
	responseTimeMs: number | null;
	lastChecked: string | null;
	lastStateChange: string | null;
	errorSnippet: string | null;
	healthUrl?: string | null;
	authMethod?: string;
}

export interface OIDCStatus {
	connected: boolean;
	providerName: string;
	tokenState: 'valid' | 'refreshing' | 'expired' | 'error';
	lastSuccess: string | null;
}

export interface ServiceGroup {
	name: string;
	services: Service[];
	counts: {
		healthy: number;
		unhealthy: number;
		authBlocked: number;
		unknown: number;
	};
	hasProblems: boolean;
	expanded: boolean;
}

export interface StateEventPayload {
	appVersion: string;
	services: Service[];
	k8sConnected?: boolean;
	k8sLastEvent?: string | null;
	healthCheckIntervalMs?: number;
	configErrors?: string[];
	oidcStatus?: OIDCStatus;
}

export interface K8sStatusPayload {
	k8sConnected: boolean;
	k8sLastEvent: string | null;
}
