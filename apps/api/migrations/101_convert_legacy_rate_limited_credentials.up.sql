-- No code sets 'rate_limited' any more; the server used to reset such rows on
-- every start, which also undid limits a running instance had just applied.
-- Convert the remaining legacy rows once instead.
UPDATE credentials SET status = 'active' WHERE status = 'rate_limited';
