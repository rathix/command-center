import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import ContextMenu from './ContextMenu.svelte';

function makeActions() {
	return [
		{ label: 'Force Refresh', action: vi.fn() },
		{ label: 'Copy URL', action: vi.fn() },
		{ label: 'Disabled Action', action: vi.fn(), disabled: true }
	];
}

describe('ContextMenu', () => {
	it('renders menu items with correct labels', () => {
		const actions = makeActions();
		render(ContextMenu, {
			props: { visible: true, x: 100, y: 100, actions, onClose: vi.fn() }
		});

		expect(screen.getByText('Force Refresh')).toBeInTheDocument();
		expect(screen.getByText('Copy URL')).toBeInTheDocument();
		expect(screen.getByText('Disabled Action')).toBeInTheDocument();
	});

	it('renders nothing when not visible', () => {
		const actions = makeActions();
		const { container } = render(ContextMenu, {
			props: { visible: false, x: 100, y: 100, actions, onClose: vi.fn() }
		});

		expect(container.querySelector('[data-testid="context-menu"]')).toBeNull();
	});

	it('clicking action calls its callback and closes menu', async () => {
		const actions = makeActions();
		const onClose = vi.fn();
		render(ContextMenu, {
			props: { visible: true, x: 100, y: 100, actions, onClose }
		});

		await fireEvent.click(screen.getByText('Force Refresh'));
		expect(actions[0].action).toHaveBeenCalledOnce();
		expect(onClose).toHaveBeenCalledOnce();
	});

	it('disabled actions are visually distinct and not clickable', async () => {
		const actions = makeActions();
		render(ContextMenu, {
			props: { visible: true, x: 100, y: 100, actions, onClose: vi.fn() }
		});

		const disabledButton = screen.getByText('Disabled Action').closest('button');
		expect(disabledButton).toBeDisabled();

		await fireEvent.click(disabledButton!);
		expect(actions[2].action).not.toHaveBeenCalled();
	});

	it('menu closes when clicking outside (backdrop)', async () => {
		const actions = makeActions();
		const onClose = vi.fn();
		render(ContextMenu, {
			props: { visible: true, x: 100, y: 100, actions, onClose }
		});

		const backdrop = screen.getByTestId('context-menu-backdrop');
		await fireEvent.click(backdrop);
		expect(onClose).toHaveBeenCalledOnce();
	});

	it('has role="menu" on the menu container', () => {
		const actions = makeActions();
		render(ContextMenu, {
			props: { visible: true, x: 100, y: 100, actions, onClose: vi.fn() }
		});

		expect(screen.getByRole('menu')).toBeInTheDocument();
	});

	it('menu items have role="menuitem"', () => {
		const actions = makeActions();
		render(ContextMenu, {
			props: { visible: true, x: 100, y: 100, actions, onClose: vi.fn() }
		});

		const menuItems = screen.getAllByRole('menuitem');
		expect(menuItems).toHaveLength(3);
	});
});
