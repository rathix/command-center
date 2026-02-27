import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { svelteTesting } from '@testing-library/svelte/vite';
import { VitePWA } from 'vite-plugin-pwa';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit(),
		VitePWA({
			registerType: 'autoUpdate',
			strategies: 'generateSW',
			scope: '/',
			base: '/',
			manifest: {
				name: 'Command Center',
				short_name: 'CmdCenter',
				description: 'Kubernetes service dashboard for homelab operators',
				theme_color: '#1e1e2e',
				background_color: '#1e1e2e',
				display: 'standalone',
				orientation: 'any',
				start_url: '/',
				categories: ['utilities'],
				icons: [
					{
						src: '/icons/icon-192x192.png',
						sizes: '192x192',
						type: 'image/png'
					},
					{
						src: '/icons/icon-512x512.png',
						sizes: '512x512',
						type: 'image/png'
					},
					{
						src: '/icons/maskable-icon-512x512.png',
						sizes: '512x512',
						type: 'image/png',
						purpose: 'maskable'
					}
				]
			},
			workbox: {
				globPatterns: ['**/*.{js,css,html,woff2,png,svg,ico}'],
				navigateFallback: 'index.html',
				navigateFallbackDenylist: [/^\/api\//],
				runtimeCaching: [
					{
						urlPattern: /^https?:\/\/[^/]+\/(?!api\/).*\.(?:js|css|woff2|png|svg|ico)$/,
						handler: 'StaleWhileRevalidate',
						options: {
							cacheName: 'static-assets'
						}
					},
					{
						urlPattern: /^https?:\/\/[^/]+\/api\/(?!events)/,
						handler: 'NetworkFirst',
						options: {
							cacheName: 'api-data',
							networkTimeoutSeconds: 3
						}
					}
				]
			}
		}),
		svelteTesting()
	],
	test: {
		environment: 'jsdom',
		setupFiles: ['./vitest-setup.ts'],
		include: ['src/**/*.{test,spec}.{js,ts}'],
		css: true
	}
});
