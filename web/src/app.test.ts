import { describe, it, expect } from 'vitest';
import css from './app.css?inline';


describe('app.css design system', () => {
	it('defines prefers-reduced-motion media query', () => {
		expect(css).toContain('@media (prefers-reduced-motion: reduce)');
		expect(css).toContain('transition-duration: 0.01ms');
		expect(css).toContain('animation-duration: 0.01ms');
	});

	it('defines Catppuccin Mocha base colors in @theme', () => {
		expect(css).toContain('--color-base: #1e1e2e');
		expect(css).toContain('--color-mantle: #181825');
		expect(css).toContain('--color-crust: #11111b');
	});

	it('defines health state tokens in @theme', () => {
		expect(css).toContain('--color-health-ok: #a6e3a1');
		expect(css).toContain('--color-health-error: #f38ba8');
		expect(css).toContain('--color-health-auth-blocked: #f9e2af');
		expect(css).toContain('--color-health-unknown: #7f849c');
	});

	it('defines JetBrains Mono font-face', () => {
		expect(css).toContain('@font-face');
		expect(css).toContain('JetBrains Mono');
		expect(css).toContain('font-weight: 100 800');
		expect(css).toContain('font-display: swap');
	});

	it('sets mono font as default body font', () => {
		expect(css).toContain('--font-mono');
		expect(css).toContain("font-family: var(--font-mono)");
	});
});
