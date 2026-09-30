ALTER TABLE mutation_proposals ADD COLUMN baseline_score REAL;
ALTER TABLE mutation_proposals ADD COLUMN candidate_score REAL;
ALTER TABLE mutation_proposals ADD COLUMN evaluation_status TEXT NOT NULL DEFAULT 'pending'
  CHECK (evaluation_status IN ('pending', 'passed', 'failed'));
