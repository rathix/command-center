import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import ConfirmationModal from './ConfirmationModal.svelte';

describe('ConfirmationModal', () => {
	const defaultProps = {
		title: 'Confirm Reboot',
		message: 'Are you sure you want to reboot this node?',
		details: { Node: 'node-01', Status: 'ready', Role: 'worker' },
		confirmLabel: 'Reboot',
		confirmVariant: 'danger' as const,
		onConfirm: vi.fn(),
		onCancel: vi.fn()
	};

	it('renders title, message, details key-value pairs', () => {
		render(ConfirmationModal, { props: defaultProps });

		expect(screen.getByText('Confirm Reboot')).toBeInTheDocument();
		expect(screen.getByText('Are you sure you want to reboot this node?')).toBeInTheDocument();
		expect(screen.getByText('Node')).toBeInTheDocument();
		expect(screen.getByText('node-01')).toBeInTheDocument();
		expect(screen.getByText('Status')).toBeInTheDocument();
		expect(screen.getByText('ready')).toBeInTheDocument();
		expect(screen.getByText('Role')).toBeInTheDocument();
		expect(screen.getByText('worker')).toBeInTheDocument();
	});

	it('confirm button calls onConfirm callback', async () => {
		const onConfirm = vi.fn();
		render(ConfirmationModal, { props: { ...defaultProps, onConfirm } });

		const confirmBtn = screen.getByText('Reboot');
		await fireEvent.click(confirmBtn);

		expect(onConfirm).toHaveBeenCalledOnce();
	});

	it('cancel button calls onCancel callback', async () => {
		const onCancel = vi.fn();
		render(ConfirmationModal, { props: { ...defaultProps, onCancel } });

		const cancelBtn = screen.getByText('Cancel');
		await fireEvent.click(cancelBtn);

		expect(onCancel).toHaveBeenCalledOnce();
	});

	it('Escape key calls onCancel', async () => {
		const onCancel = vi.fn();
		render(ConfirmationModal, { props: { ...defaultProps, onCancel } });

		await fireEvent.keyDown(window, { key: 'Escape' });

		expect(onCancel).toHaveBeenCalledOnce();
	});

	it('confirm button disabled when disabled prop is true', () => {
		render(ConfirmationModal, { props: { ...defaultProps, disabled: true } });

		const confirmBtn = screen.getByText('Reboot');
		expect(confirmBtn).toBeDisabled();
	});

	it('confirm button not disabled by default', () => {
		render(ConfirmationModal, { props: defaultProps });

		const confirmBtn = screen.getByText('Reboot');
		expect(confirmBtn).not.toBeDisabled();
	});

	it('has role=dialog and aria-modal', () => {
		render(ConfirmationModal, { props: defaultProps });

		const dialog = screen.getByRole('dialog');
		expect(dialog).toHaveAttribute('aria-modal', 'true');
	});
});
