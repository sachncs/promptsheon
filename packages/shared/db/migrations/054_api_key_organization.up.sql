-- Bind every newly issued API key to one organization so callers cannot
-- select tenant scope with a request header.
ALTER TABLE api_keys ADD COLUMN organization_id TEXT REFERENCES orgs(id) ON DELETE CASCADE;

UPDATE api_keys
SET organization_id = (
  SELECT m.org_id
  FROM org_members m
  WHERE m.user_id = api_keys.user_id
  ORDER BY m.joined_at ASC
  LIMIT 1
)
WHERE organization_id IS NULL;

-- Keys that cannot be deterministically assigned to an organization are
-- revoked rather than left usable with ambiguous tenant scope.
UPDATE api_keys SET revoked = 1 WHERE organization_id IS NULL;

CREATE INDEX idx_api_keys_organization ON api_keys(organization_id, created_at DESC);
