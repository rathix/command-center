import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

describe('PWA configuration', () => {
	it('vite.config.ts contains VitePWA plugin import', () => {
		const configPath = path.resolve(__dirname, '../../vite.config.ts');
		const content = fs.readFileSync(configPath, 'utf-8');
		expect(content).toContain("import { VitePWA } from 'vite-plugin-pwa'");
	});

	it('manifest includes required fields in vite config', () => {
		const configPath = path.resolve(__dirname, '../../vite.config.ts');
		const content = fs.readFileSync(configPath, 'utf-8');
		expect(content).toContain("name: 'Command Center'");
		expect(content).toContain("short_name: 'CmdCenter'");
		expect(content).toContain("theme_color: '#1e1e2e'");
		expect(content).toContain("background_color: '#1e1e2e'");
		expect(content).toContain("display: 'standalone'");
		expect(content).toContain("start_url: '/'");
	});

	it('manifest includes required icon sizes', () => {
		const configPath = path.resolve(__dirname, '../../vite.config.ts');
		const content = fs.readFileSync(configPath, 'utf-8');
		expect(content).toContain("sizes: '192x192'");
		expect(content).toContain("sizes: '512x512'");
		expect(content).toContain("purpose: 'maskable'");
	});

	it('icon files referenced in manifest exist in static directory', () => {
		const staticDir = path.resolve(__dirname, '../../static/icons');
		const requiredIcons = [
			'icon-192x192.png',
			'icon-512x512.png',
			'maskable-icon-512x512.png',
			'apple-touch-icon.png'
		];

		for (const icon of requiredIcons) {
			const iconPath = path.join(staticDir, icon);
			expect(fs.existsSync(iconPath), `${icon} should exist in static/icons/`).toBe(true);
		}
	});

	it('app.html includes PWA meta tags', () => {
		const htmlPath = path.resolve(__dirname, '../app.html');
		const content = fs.readFileSync(htmlPath, 'utf-8');
		expect(content).toContain('name="theme-color" content="#1e1e2e"');
		expect(content).toContain('name="apple-mobile-web-app-capable" content="yes"');
		expect(content).toContain('name="apple-mobile-web-app-status-bar-style" content="black-translucent"');
		expect(content).toContain('rel="apple-touch-icon"');
	});

	it('workbox is configured with generateSW strategy', () => {
		const configPath = path.resolve(__dirname, '../../vite.config.ts');
		const content = fs.readFileSync(configPath, 'utf-8');
		expect(content).toContain("strategies: 'generateSW'");
		expect(content).toContain("registerType: 'autoUpdate'");
	});

	it('workbox excludes SSE endpoint from caching', () => {
		const configPath = path.resolve(__dirname, '../../vite.config.ts');
		const content = fs.readFileSync(configPath, 'utf-8');
		// The navigateFallbackDenylist should prevent /api/ paths from getting the fallback
		expect(content).toContain('navigateFallbackDenylist');
		// SSE endpoint (/api/events) excluded via the urlPattern that skips /api/events
		expect(content).toContain('api\\/(?!events)');
	});

	it('service worker registration is in +layout.svelte', () => {
		const layoutPath = path.resolve(__dirname, '../routes/+layout.svelte');
		const content = fs.readFileSync(layoutPath, 'utf-8');
		expect(content).toContain("import('virtual:pwa-register')");
		expect(content).toContain('registerSW');
		expect(content).toContain("'serviceWorker' in navigator");
	});
});
