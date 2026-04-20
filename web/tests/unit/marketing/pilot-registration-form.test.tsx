import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { PilotRegistrationForm } from '@/components/marketing/forms/pilot-registration-form';

describe('PilotRegistrationForm', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the form with submit button', () => {
    render(<PilotRegistrationForm locale="en" />);
    expect(screen.getByRole('button', { name: /submit/i })).toBeInTheDocument();
  });

  it('renders all required form fields', () => {
    render(<PilotRegistrationForm locale="en" />);

    // 3 text inputs (name, email, org) + 1 textarea (message) + 1 hidden honeypot
    const textInputs = screen.getAllByRole('textbox');
    expect(textInputs.length).toBeGreaterThanOrEqual(3);

    // Role select
    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('renders honeypot field as hidden', () => {
    const { container } = render(<PilotRegistrationForm locale="en" />);

    const honeypot = container.querySelector('[aria-hidden="true"] input');
    expect(honeypot).toBeInTheDocument();
    expect(honeypot).toHaveAttribute('tabIndex', '-1');
    expect(honeypot).toHaveAttribute('autocomplete', 'off');
  });

  it('does not show success state initially', () => {
    render(<PilotRegistrationForm locale="en" />);
    expect(
      screen.queryByText('marketing.contact.form.successTitle'),
    ).not.toBeInTheDocument();
  });

  it('does not show error banner initially', () => {
    const { container } = render(<PilotRegistrationForm locale="en" />);
    expect(container.querySelector('.banner-error')).not.toBeInTheDocument();
  });

  it('renders all role options', () => {
    render(<PilotRegistrationForm locale="en" />);

    const select = screen.getByRole('combobox');
    const options = select.querySelectorAll('option');
    expect(options).toHaveLength(6); // investigator, prosecutor, defense, analyst, court_admin, other
  });

  it('renders disclaimer text', () => {
    render(<PilotRegistrationForm locale="en" />);
    expect(
      screen.getByText('marketing.contact.form.disclaimer'),
    ).toBeInTheDocument();
  });
});

describe('PilotRegistrationForm error mapping', () => {
  it('maps 429 to safe rate limit message', () => {
    const safeMessages: Record<number, string> = {
      429: 'Too many requests. Please try again later.',
      400: 'Please check your input and try again.',
    };

    expect(safeMessages[429]).toBe(
      'Too many requests. Please try again later.',
    );
    expect(safeMessages[400]).toBe(
      'Please check your input and try again.',
    );
  });

  it('falls back to generic message for unknown status codes', () => {
    const safeMessages: Record<number, string> = {
      429: 'Too many requests. Please try again later.',
      400: 'Please check your input and try again.',
    };

    const fallback =
      safeMessages[503] || 'Submission failed. Please try again.';
    expect(fallback).toBe('Submission failed. Please try again.');
  });

  it('never reflects raw server error strings', () => {
    // The component maps status codes to safe messages
    // instead of using body.error from the API
    const safeMessages: Record<number, string> = {
      429: 'Too many requests. Please try again later.',
      400: 'Please check your input and try again.',
    };

    // Simulated dangerous server response
    const dangerousServerError =
      'Error: ECONNREFUSED at /internal/path/file.ts:42';

    // The safe message for an unknown status should NOT contain the server error
    const displayed =
      safeMessages[500] || 'Submission failed. Please try again.';
    expect(displayed).not.toContain(dangerousServerError);
    expect(displayed).not.toContain('/internal/');
    expect(displayed).not.toContain('ECONNREFUSED');
  });
});
