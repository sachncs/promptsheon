-- Migration 064 down: this migration is destructive (clears
-- plaintext secrets). The down migration cannot recover them
-- because they were already encrypted or lost. Document the
-- operator-visible fact.
SELECT 1;
