export type HealthStatus = 'healthy' | 'unhealthy' | 'authBlocked' | 'unknown';

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

export interface Service {
	name: string;
	displayName: string;
	namespace: string;
	url: string;
	status: HealthStatus;
	httpCode: number | null;
	responseTimeMs: number | null;
	lastChecked: string | null;
	lastStateChange: string | null;
	errorSnippet: string | null;
}

export interface StateEventPayload {
	appVersion: string;
	services: Service[];
	k8sConnected?: boolean;
	k8sLastEvent?: string | null;
	healthCheckIntervalMs?: number;
}

export interface K8sStatusPayload {
	k8sConnected: boolean;
	k8sLastEvent: string | null;
}
