/** Unwraps list responses sent as {object: "list", data: [...]} or as a bare array. */
export function listData<T>(body: T[] | { data?: T[] } | null | undefined): T[] {
  if (Array.isArray(body)) return body;
  return body?.data ?? [];
}
