-- Also created automatically by db.Store.migrate() on startup;
-- kept here as an explicit, reviewable migration for CI/CD pipelines.
CREATE TABLE IF NOT EXISTS reports (
    id          SERIAL PRIMARY KEY,
    repo_path   TEXT NOT NULL,
    report_json JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reports_repo_path ON reports (repo_path);
