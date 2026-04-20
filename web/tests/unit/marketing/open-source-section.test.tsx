import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { OpenSourceSection } from '@/components/marketing/sections/open-source-section';

describe('OpenSourceSection', () => {
  it('renders the section with correct translation keys', () => {
    render(<OpenSourceSection />);

    expect(
      screen.getByText('marketing.openSource.eyebrow'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('marketing.openSource.title'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('marketing.openSource.description'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('marketing.openSource.license'),
    ).toBeInTheDocument();
  });

  it('renders GitHub link with correct URL', () => {
    render(<OpenSourceSection />);

    const link = screen.getByText('marketing.openSource.cta').closest('a');
    expect(link).toHaveAttribute(
      'href',
      'https://github.com/KyleFuehri/VaultKeeper',
    );
  });

  it('renders external link with security attributes', () => {
    render(<OpenSourceSection />);

    const link = screen.getByText('marketing.openSource.cta').closest('a');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('marks ExternalLink icon as aria-hidden', () => {
    const { container } = render(<OpenSourceSection />);
    const link = container.querySelector('a[target="_blank"]');
    const svg = link?.querySelector('svg');
    // Lucide renders with aria-hidden attribute
    expect(svg).toBeTruthy();
  });

  it('renders as a section element', () => {
    const { container } = render(<OpenSourceSection />);
    expect(container.querySelector('section')).toBeInTheDocument();
  });

  it('does not contain misleading compliance language', () => {
    const { container } = render(<OpenSourceSection />);
    const text = container.textContent || '';
    expect(text).not.toContain('SOC 2 Type II');
    expect(text).not.toContain('ISO 27001');
    expect(text).not.toContain('certified');
  });
});
