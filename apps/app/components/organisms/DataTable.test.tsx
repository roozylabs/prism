import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen, within, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DataTable, type Column } from './DataTable';

type Row = { id: string; name: string; requests: number };

const rows: Row[] = Array.from({ length: 25 }, (_, i) => ({ id: `k${i + 1}`, name: `key-${String(i + 1).padStart(2, '0')}`, requests: 25 - i }));
const columns: Column<Row>[] = [
  { title: 'Name', dataIndex: 'name', key: 'name' },
  { title: 'Requests', dataIndex: 'requests', key: 'requests' },
];

// The footer repeats the page number in "Page N of M", so assertions go
// through the labelled buttons and the "Total: N items" line.
function totalLine(n: number) {
  return screen.getByText((_, el) => el?.tagName === 'SPAN' && el.textContent === `Total: ${n} items`);
}

function currentPage(n: number) {
  return screen.getByRole('button', { name: `Page ${n}` });
}

function bodyNames() {
  const body = screen.getAllByRole('rowgroup')[1];
  return within(body)
    .getAllByRole('row')
    .map((r) => within(r).getAllByRole('cell')[0].textContent);
}

describe('DataTable (client paging)', () => {
  afterEach(() => vi.useRealTimers());

  it('pages, searches and sorts the rows it is given', async () => {
    const user = userEvent.setup();
    render(<DataTable dataSource={rows} columns={columns} rowKey="id" pageSize={10} />);

    expect(totalLine(25)).toBeInTheDocument();
    expect(bodyNames()).toHaveLength(10);
    expect(bodyNames()[0]).toBe('key-01');

    await user.click(screen.getByRole('button', { name: 'Next page' }));
    expect(currentPage(2)).toHaveAttribute('aria-current', 'page');
    expect(bodyNames()[0]).toBe('key-11');
    await user.click(screen.getByRole('button', { name: 'Page 3' }));
    expect(bodyNames()).toHaveLength(5);
    expect(screen.getByRole('button', { name: 'Next page' })).toBeDisabled();

    await user.type(screen.getByPlaceholderText('Search records...'), 'key-2');
    expect(bodyNames()).toEqual(['key-20', 'key-21', 'key-22', 'key-23', 'key-24', 'key-25']);
    expect(totalLine(6)).toBeInTheDocument();
    await user.clear(screen.getByPlaceholderText('Search records...'));

    // Sorting by requests: ascending, then descending, then off.
    await user.click(screen.getByText('Requests'));
    expect(bodyNames()[0]).toBe('key-25');
    await user.click(screen.getByText('Requests'));
    expect(bodyNames()[0]).toBe('key-01');
    await user.click(screen.getByText('Requests'));
    expect(bodyNames()[0]).toBe('key-01');
  });

  it('shows the empty text and the loading skeleton', () => {
    const { rerender } = render(<DataTable dataSource={[]} columns={columns} rowKey="id" emptyText="Nothing here" />);
    expect(screen.getByText('Nothing here')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Next page' })).not.toBeInTheDocument();

    rerender(<DataTable dataSource={[]} columns={columns} rowKey="id" loading />);
    expect(screen.queryByText('Nothing here')).not.toBeInTheDocument();
    expect(screen.getAllByRole('rowgroup')[1].querySelectorAll('tr')).toHaveLength(5);
  });
});

describe('DataTable (server paging)', () => {
  it('reports the API totals and hands paging and search to the parent', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const onPageChange = vi.fn();
    const onSearchChange = vi.fn();
    render(
      <DataTable
        dataSource={rows.slice(0, 20)}
        columns={columns}
        rowKey="id"
        pageSize={20}
        server={{ total: 95, page: 2, onPageChange, onSearchChange }}
      />,
    );

    // One page of 20 is shown as page 2 of 5 for a total of 95.
    expect(totalLine(95)).toBeInTheDocument();
    expect(currentPage(2)).toHaveAttribute('aria-current', 'page');
    expect(currentPage(5)).not.toHaveAttribute('aria-current');
    expect(bodyNames()).toHaveLength(20);

    await user.click(screen.getByRole('button', { name: 'Next page' }));
    expect(onPageChange).toHaveBeenLastCalledWith(3);
    await user.click(screen.getByRole('button', { name: 'Page 5' }));
    expect(onPageChange).toHaveBeenLastCalledWith(5);

    // Typing does not filter locally; the parent is told after a pause.
    await user.type(screen.getByPlaceholderText('Search records...'), 'zzz');
    expect(bodyNames()).toHaveLength(20);
    expect(onPageChange).toHaveBeenLastCalledWith(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(350);
    });
    expect(onSearchChange).toHaveBeenLastCalledWith('zzz');
    vi.useRealTimers();
  });
});
