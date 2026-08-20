-- 038_redteam.up.sql
-- Phase 2.7: red-team pack tables. Each pack is a curated corpus
-- of attack patterns; a run is a suite execution that scores
-- each case on resistance (lower = worse).

CREATE TABLE redteam_packs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    category TEXT NOT NULL CHECK (category IN ('injection', 'jailbreak', 'exfil')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE redteam_cases (
    id TEXT PRIMARY KEY,
    pack_id TEXT NOT NULL REFERENCES redteam_packs(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    prompt TEXT NOT NULL,
    expected_refusal_match TEXT,
    severity TEXT NOT NULL DEFAULT 'medium' CHECK (severity IN ('low', 'medium', 'high')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_redteam_cases_pack ON redteam_cases(pack_id);

CREATE TABLE redteam_runs (
    id TEXT PRIMARY KEY,
    pack_id TEXT NOT NULL REFERENCES redteam_packs(id) ON DELETE CASCADE,
    run_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resistance REAL NOT NULL,
    results TEXT NOT NULL
);

CREATE INDEX idx_redteam_runs_pack ON redteam_runs(pack_id);

-- Seed: three core packs with a small initial corpus. The
-- evaluation engine can extend the corpus by ingesting JSON
-- files dropped into .promptsheon/redteam/.
INSERT INTO redteam_packs (id, name, description, category) VALUES
    ('rt-injection', 'prompt-injection', 'Direct instruction overrides', 'injection'),
    ('rt-jailbreak', 'jailbreak', 'Role-play and persona-shifted bypass', 'jailbreak'),
    ('rt-exfil', 'data-exfiltration', 'Probe for tool / data leak surfaces', 'exfil');
