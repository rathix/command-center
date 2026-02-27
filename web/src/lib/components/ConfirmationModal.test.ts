import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ConfirmationModalTest from './ConfirmationModalTest.svelte';

describe('ConfirmationModal', () => {
	it('renders title and body content when open', () => {
		render(ConfirmationModalTest, {
			props: { open: true, title: 'Confirm Delete', bodyText: 'Are you sure?', loading: false }
		});

		expect(screen.getByText('Confirm Delete')).toBeInTheDocument();
		expect(screen.getByText('Are you sure?')).toBeInTheDocument();
	});

	it('does not render when closed', () => {
		render(ConfirmationModalTest, {
			props: { open: false, title: 'Confirm Delete', bodyText: 'Are you sure?', loading: false }
		});

		expect(screen.queryByTestId('confirmation-modal')).not.toBeInTheDocument();
	});

	it('calls onconfirm when Confirm clicked', async () => {
		render(ConfirmationModalTest, {
			props: { open: true, title: 'Confirm', bodyText: 'Proceed?', loading: false }
		});

		const confirmBtn = screen.getByTestId('confirm-button');
		await fireEvent.click(confirmBtn);

		expect(screen.getByTestId('confirmed-flag')).toBeInTheDocument();
	});

	it('calls oncancel when Cancel clicked', async () => {
		render(ConfirmationModalTest, {
			props: { open: true, title: 'Confirm', bodyText: 'Proceed?', loading: false }
		});

		const cancelBtn = screen.getByTestId('cancel-button');
		await fireEvent.click(cancelBtn);

		expect(screen.getByTestId('cancelled-flag')).toBeInTheDocument();
	});

	it('calls oncancel when Escape pressed', async () => {
		render(ConfirmationModalTest, {
			props: { open: true, title: 'Confirm', bodyText: 'Proceed?', loading: false }
		});

		await fireEvent.keyDown(window, { key: 'Escape' });

		expect(screen.getByTestId('cancelled-flag')).toBeInTheDocument();
	});

	it('Confirm button shows loading state when in-flight', () => {
		render(ConfirmationModalTest, {
			props: { open: true, title: 'Confirm', bodyText: 'Proceed?', loading: true }
		});

		const confirmBtn = screen.getByTestId('confirm-button');
		expect(confirmBtn).toBeDisabled();
		expect(screen.getByText('Processing...')).toBeInTheDocument();
	});

	it('has dialog role and aria-modal', () => {
		render(ConfirmationModalTest, {
			props: { open: true, title: 'Confirm', bodyText: 'Proceed?', loading: false }
		});

		const dialog = screen.getByRole('dialog');
		expect(dialog).toHaveAttribute('aria-modal', 'true');
	});
});
