import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { CategoriesResponse } from '../types';
import { CategoryLegend } from './CategoryLegend';

const categories: CategoriesResponse = {
  total: { usage: 10240, size: 10240 },
  categories: [
    { id: 'build-artifact', label: 'Build artifacts', usage: 2048, size: 2048 },
    { id: 'ml-model', label: 'ML models', usage: 3072, size: 3072 },
    { id: 'installer', label: 'Installers & archives', usage: 0, size: 0 },
  ],
};

afterEach(cleanup);

describe('CategoryLegend', () => {
  it('shows reclaimable totals and only present categories', () => {
    render(
      <CategoryLegend
        projection="cleanup"
        categories={categories}
        apparent={false}
        useSIPrefix={false}
        filter={null}
        onFilter={() => undefined}
      />,
    );
    expect(screen.getByText('Candidates: 5.0 KiB (50%)')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Build artifacts · 2.0 KiB · 20%/ })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Installers/ })).toBeNull();
    expect(screen.getByText('safe')).toBeTruthy();
  });

  it('toggles the category filter', () => {
    const onFilter = vi.fn();
    const { rerender } = render(
      <CategoryLegend
        projection="categories"
        categories={categories}
        apparent={false}
        useSIPrefix={false}
        filter={null}
        onFilter={onFilter}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: /ML models/ }));
    expect(onFilter).toHaveBeenLastCalledWith('ml-model');

    rerender(
      <CategoryLegend
        projection="categories"
        categories={categories}
        apparent={false}
        useSIPrefix={false}
        filter="ml-model"
        onFilter={onFilter}
      />,
    );
    const chip = screen.getByRole('button', { name: /ML models/ });
    expect(chip.getAttribute('aria-pressed')).toBe('true');
    fireEvent.click(chip);
    expect(onFilter).toHaveBeenLastCalledWith(null);
  });

  it('says when a directory has no candidates', () => {
    render(
      <CategoryLegend
        projection="cleanup"
        categories={{ total: { usage: 1, size: 1 }, categories: [] }}
        apparent={false}
        useSIPrefix={false}
        filter={null}
        onFilter={() => undefined}
      />,
    );
    expect(screen.getByText('No cleanup candidates here')).toBeTruthy();
  });

  it('explains the age projection and renders nothing for location', () => {
    const { container, rerender } = render(
      <CategoryLegend
        projection="age"
        categories={null}
        apparent={false}
        useSIPrefix={false}
        filter={null}
        onFilter={() => undefined}
      />,
    );
    expect(screen.getByText('older')).toBeTruthy();

    rerender(
      <CategoryLegend
        projection="location"
        categories={null}
        apparent={false}
        useSIPrefix={false}
        filter={null}
        onFilter={() => undefined}
      />,
    );
    expect(container.innerHTML).toBe('');
  });
});
