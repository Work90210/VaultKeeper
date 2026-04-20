import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { FaqSection } from '@/components/marketing/sections/faq-section';

describe('FaqSection', () => {
  it('renders all 8 FAQ items', () => {
    render(<FaqSection />);

    const buttons = screen.getAllByRole('button');
    expect(buttons).toHaveLength(8);
  });

  it('defaults sovereignty, security, and pilot items to open', () => {
    render(<FaqSection />);

    const buttons = screen.getAllByRole('button');

    // sovereignty = index 0, security = index 1, pilot = index 3
    expect(buttons[0]).toHaveAttribute('aria-expanded', 'true');
    expect(buttons[1]).toHaveAttribute('aria-expanded', 'true');
    expect(buttons[3]).toHaveAttribute('aria-expanded', 'true');

    // Others should be closed
    expect(buttons[2]).toHaveAttribute('aria-expanded', 'false');
    expect(buttons[4]).toHaveAttribute('aria-expanded', 'false');
    expect(buttons[5]).toHaveAttribute('aria-expanded', 'false');
    expect(buttons[6]).toHaveAttribute('aria-expanded', 'false');
    expect(buttons[7]).toHaveAttribute('aria-expanded', 'false');
  });

  it('supports multi-open — toggling one does not close others', () => {
    render(<FaqSection />);

    const buttons = screen.getAllByRole('button');

    // Close sovereignty (index 0)
    fireEvent.click(buttons[0]);
    expect(buttons[0]).toHaveAttribute('aria-expanded', 'false');

    // Security and pilot should still be open
    expect(buttons[1]).toHaveAttribute('aria-expanded', 'true');
    expect(buttons[3]).toHaveAttribute('aria-expanded', 'true');
  });

  it('can open a closed item', () => {
    render(<FaqSection />);

    const buttons = screen.getAllByRole('button');

    // Integration (index 2) starts closed
    expect(buttons[2]).toHaveAttribute('aria-expanded', 'false');

    // Open it
    fireEvent.click(buttons[2]);
    expect(buttons[2]).toHaveAttribute('aria-expanded', 'true');
  });

  it('can close an open item', () => {
    render(<FaqSection />);

    const buttons = screen.getAllByRole('button');

    // Security (index 1) starts open
    expect(buttons[1]).toHaveAttribute('aria-expanded', 'true');

    // Close it
    fireEvent.click(buttons[1]);
    expect(buttons[1]).toHaveAttribute('aria-expanded', 'false');
  });

  it('has aria-controls on each button', () => {
    render(<FaqSection />);

    const buttons = screen.getAllByRole('button');
    buttons.forEach((button) => {
      expect(button).toHaveAttribute('aria-controls');
      expect(button.getAttribute('aria-controls')).toMatch(/^faq-panel-/);
    });
  });

  it('has matching id on panels for open items', () => {
    const { container } = render(<FaqSection />);

    const regions = container.querySelectorAll('[role="region"]');
    expect(regions.length).toBeGreaterThan(0);

    regions.forEach((region) => {
      expect(region).toHaveAttribute('aria-labelledby');
      expect(region.getAttribute('aria-labelledby')).toMatch(
        /^faq-trigger-/,
      );
      expect(region).toHaveAttribute('id');
      expect(region.getAttribute('id')).toMatch(/^faq-panel-/);
    });
  });

  it('button id and panel aria-labelledby match', () => {
    const { container } = render(<FaqSection />);

    const buttons = screen.getAllByRole('button');
    buttons.forEach((button) => {
      const triggerId = button.getAttribute('id');
      const panelId = button.getAttribute('aria-controls');

      if (button.getAttribute('aria-expanded') === 'true') {
        const panel = container.querySelector(`#${panelId}`);
        expect(panel).toBeInTheDocument();
        expect(panel?.getAttribute('aria-labelledby')).toBe(triggerId);
      }
    });
  });
});
