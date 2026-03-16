import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Docs } from '@/views/Docs';

function renderDocs() {
  return render(<Docs />);
}

describe('Docs', () => {
  it('renders the Overview page by default', () => {
    renderDocs();
    expect(screen.getByText('Documentation')).toBeInTheDocument();
    // Overview heading inside content area
    expect(
      screen.getByRole('heading', { name: 'Overview', level: 1 }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/ArgoCD Addons Platform/),
    ).toBeInTheDocument();
  });

  it('renders all navigation links', () => {
    renderDocs();
    const expectedIds = [
      'overview',
      'features',
      'managing-addons',
      'values-guide',
      'troubleshooting',
    ];

    for (const id of expectedIds) {
      expect(screen.getByTestId(`doc-nav-${id}`)).toBeInTheDocument();
    }
  });

  it('does not render removed pages', () => {
    renderDocs();
    expect(screen.queryByTestId('doc-nav-architecture')).not.toBeInTheDocument();
    expect(screen.queryByTestId('doc-nav-adding-cluster')).not.toBeInTheDocument();
  });

  it('navigates to another doc page when clicked', async () => {
    const user = userEvent.setup();
    renderDocs();

    await user.click(screen.getByTestId('doc-nav-features'));

    expect(
      screen.getByRole('heading', { name: 'Features', level: 1 }),
    ).toBeInTheDocument();
    expect(screen.getByText(/quick tour/)).toBeInTheDocument();
  });
});
