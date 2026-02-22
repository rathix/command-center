export type HealthStatus = 'healthy' | 'degraded' | 'unhealthy' | 'unknown';
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
	'degraded',
	'unhealthy',
	'unknown'
];

export const DEFAULT_HEALTH_CHECK_INTERVAL_MS = 30_000;

export interface PodDiagnostic {
	reason: string | null;
	restartCount: number;
}

export interface Service {
	name: string;
	icon?: string | null;
	displayName: string;
	namespace: string;
	group: string;
	url: string;
	source?: ServiceSource;
	status: HealthStatus;
	compositeStatus: HealthStatus;
	authGuarded: boolean;
	httpCode: number | null;
	responseTimeMs: number | null;
	lastChecked: string | null;
	lastStateChange: string | null;
	errorSnippet: string | null;
	podDiagnostic: PodDiagnostic | null;
	healthUrl?: string | null;
	readyEndpoints: number | null;
	totalEndpoints: number | null;
}

export interface ServiceGroup {
	name: string;
	services: Service[];
	counts: {
		healthy: number;
		degraded: number;
		unhealthy: number;
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
}

export interface K8sStatusPayload {
	k8sConnected: boolean;
	k8sLastEvent: string | null;
}
