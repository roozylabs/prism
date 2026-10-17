-- Migration 109: store session tokens as SHA-256 hashes.
-- The session table held the exact bearer value the cookie carries, so a
-- database read or a backup gave away every live session. The API now looks
-- sessions up by the hash of the presented token; existing rows are hashed
-- in place so current cookies keep working.
UPDATE session SET token = encode(sha256(convert_to(token, 'UTF8')), 'hex');
