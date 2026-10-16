import { PaginatedResult } from './types/common';

/** Largest page the API serves (httputil.MaxPageSize). */
export const MAX_PAGE_SIZE = 200;

/**
 * Collects every page of a list, for pickers and dropdowns that need all
 * items rather than one page. Stops after maxPages as a safety net.
 */
export async function fetchAllPages<T>(
  fetchPage: (page: number, limit: number) => Promise<PaginatedResult<T>>,
  maxPages = 50,
): Promise<T[]> {
  const items: T[] = [];
  for (let page = 1; page <= maxPages; page++) {
    const result = await fetchPage(page, MAX_PAGE_SIZE);
    items.push(...(result.data ?? []));
    if (page >= (result.totalPages ?? 1)) break;
  }
  return items;
}
