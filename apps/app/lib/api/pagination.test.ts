import { describe, expect, it, vi } from 'vitest';
import { fetchAllPages, MAX_PAGE_SIZE } from './pagination';
import { listData } from './envelope';
import type { PaginatedResult } from './types/common';

function page<T>(data: T[], pageNo: number, totalPages: number): PaginatedResult<T> {
  return { data, page: pageNo, limit: MAX_PAGE_SIZE, total: totalPages * data.length, totalPages };
}

describe('fetchAllPages', () => {
  it('walks every page at the largest page size and concatenates the rows', async () => {
    const fetchPage = vi.fn(async (p: number) => page([`${p}-a`, `${p}-b`], p, 3));

    const items = await fetchAllPages(fetchPage);

    expect(items).toEqual(['1-a', '1-b', '2-a', '2-b', '3-a', '3-b']);
    expect(fetchPage).toHaveBeenCalledTimes(3);
    expect(fetchPage).toHaveBeenNthCalledWith(1, 1, MAX_PAGE_SIZE);
    expect(fetchPage).toHaveBeenNthCalledWith(3, 3, MAX_PAGE_SIZE);
  });

  it('stops after one page when the API reports a single page or omits the count', async () => {
    const single = vi.fn(async () => page(['x'], 1, 1));
    expect(await fetchAllPages(single)).toEqual(['x']);
    expect(single).toHaveBeenCalledTimes(1);

    const noCount = vi.fn(async () => ({ data: ['y'] }) as unknown as PaginatedResult<string>);
    expect(await fetchAllPages(noCount)).toEqual(['y']);
    expect(noCount).toHaveBeenCalledTimes(1);
  });

  it('tolerates a missing data array and caps the number of pages', async () => {
    const endless = vi.fn(async (p: number) => ({ data: p % 2 ? undefined : [p], totalPages: 1000 }) as unknown as PaginatedResult<number>);

    const items = await fetchAllPages(endless, 4);

    expect(items).toEqual([2, 4]);
    expect(endless).toHaveBeenCalledTimes(4);
  });
});

describe('listData', () => {
  it('accepts a bare array, a list envelope, or nothing', () => {
    expect(listData([1, 2])).toEqual([1, 2]);
    expect(listData({ data: ['a'] })).toEqual(['a']);
    expect(listData({})).toEqual([]);
    expect(listData(null)).toEqual([]);
    expect(listData(undefined)).toEqual([]);
  });
});
