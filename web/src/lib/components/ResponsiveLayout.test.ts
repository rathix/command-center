import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import ServiceRow from './ServiceRow.svelte';
import type { Service } from '$lib/types';
import * as breakpointStore from '$lib/breakpointStore.svelte';

function makeService(overrides: Partial<Service> = {}): Service {
	return {
		name: 'grafana',
		displayName: 'grafana',
		namespace: 'monitoring',
		group: 'monitoring',
		url: 'https://grafana.example.com',
		status: 'unknown',
		compositeStatus: overrides.compositeStatus ?? overrides.status ?? 'unknown',
		readyEndpoints: null,
		totalEndpoints: null,
		authGuarded: false,
		httpCode: null,
		responseTimeMs: null,
		lastChecked: null,
		lastStateChange: null,
		errorSnippet: null,
		podDiagnostic: null,
		...overrides
	};
}

describe('ResponsiveLayout: ServiceRow responsive behavior', () => {
	beforeEach(() => {
		breakpointStore._resetForTesting();
	});

	it('ServiceRow has minimum touch target height', () => {
		render(ServiceRow, {
			props: { service: makeService(), odd: false }
		});
		const listItem = screen.getByRole('listitem');
		expect(listItem).toHaveClass('min-h-[var(--service-row-min-height)]');
	});

	it('ServiceRow interactive element has touch-action manipulation', () => {
		render(ServiceRow, {
			props: { service: makeService(), odd: false }
		});
		const button = screen.getByRole('button');
		expect(button.style.touchAction).toBe('manipulation');
	});

	it('ServiceRow has minimum touch target on interactive element', () => {
		render(ServiceRow, {
			props: { service: makeService(), odd: false }
		});
		const button = screen.getByRole('button');
		expect(button).toHaveClass('min-h-[var(--touch-target-min)]');
	});

	it('service name uses truncation class', () => {
		render(ServiceRow, {
			props: { service: makeService({ displayName: 'very-long-service-name-that-might-overflow' }), odd: false }
		});
		const nameSpan = screen.getByText('very-long-service-name-that-might-overflow');
		expect(nameSpan).toHaveClass('truncate');
	});
});

describe('ResponsiveLayout: viewport meta tag', () => {
	it('app.html contains viewport meta tag without user-scalable=no', () => {
		const fs = require('node:fs');
		const path = require('node:path');
		const htmlPath = path.resolve(__dirname, '../../app.html');
		const content = fs.readFileSync(htmlPath, 'utf-8');
		expect(content).toContain('name="viewport"');
		expect(content).toContain('width=device-width');
		expect(content).not.toContain('user-scalable=no');
		expect(content).not.toContain('maximum-scale=1');
	});
});
