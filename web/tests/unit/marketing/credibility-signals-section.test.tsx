import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { CredibilitySignalsSection } from '@/components/marketing/sections/credibility-signals-section';

describe('CredibilitySignalsSection', () => {
  it('renders all four trust cards', () => {
    render(<CredibilitySignalsSection />);

    expect(
      screen.getByText('marketing.credibilitySignals.berkeleyProtocol.title'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('marketing.credibilitySignals.rfc3161.title'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('marketing.credibilitySignals.sovereignty.title'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('marketing.credibilitySignals.auditTrails.title'),
    ).toBeInTheDocument();
  });

  it('renders descriptions for all trust cards', () => {
    render(<CredibilitySignalsSection />);

    expect(
      screen.getByText(
        'marketing.credibilitySignals.berkeleyProtocol.description',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'marketing.credibilitySignals.rfc3161.description',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'marketing.credibilitySignals.sovereignty.description',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'marketing.credibilitySignals.auditTrails.description',
      ),
    ).toBeInTheDocument();
  });

  it('renders as a section element', () => {
    const { container } = render(<CredibilitySignalsSection />);
    const section = container.querySelector('section');
    expect(section).toBeInTheDocument();
  });

  it('renders four card containers', () => {
    const { container } = render(<CredibilitySignalsSection />);
    // 4 trust cards in a grid
    const headings = container.querySelectorAll('h3');
    expect(headings).toHaveLength(4);
  });

  it('does not contain fake stats or social proof language', () => {
    const { container } = render(<CredibilitySignalsSection />);
    const text = container.textContent || '';
    expect(text).not.toContain('trusted by');
    expect(text).not.toContain('Trusted by');
    expect(text).not.toContain('worldwide');
  });
});
