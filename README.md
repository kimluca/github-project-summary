# 🏺 Codebase Archaeologist

Point it at any git repository on disk and it tells you, in seconds, what
you're actually looking at: which languages dominate, what dependency
managers are in play, and — using git history — which files are both large
*and* frequently touched, i.e. the files worth reading first when you
inherit an unfamiliar codebase.

## Stack

- **Go** backend, standard library `net/http` + `os/exec` (for `git log`)
- **GraphQL** API — hand-rolled, dependency-free resolver (see
  `internal/graphql/graphql.go`) so the wire contract is 100% transparent
- **Postgres** (`lib/pq`) for caching reports (10 min TTL) and analysis history
- **TypeScript + React** (Vite) frontend, typed GraphQL client with no codegen

## Why hand-rolled GraphQL?

Most tutorials reach for `gqlgen` immediately. I wanted to actually
understand the request/response contract GraphQL clients rely on, so
`internal/graphql/graphql.go` implements just enough of the spec (single
query/mutation, flat args, variables) to serve this API — same wire format,
zero magic. Swapping in `gqlgen` later would be a drop-in replacement.

## Running it

```bash
# 1. Postgres (or use any existing instance)
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:16

# 2. Backend
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
go run ./cmd/

# 3. Frontend (separate terminal)
cd web && npm install && npm run dev
```

Open the Vite dev server URL, paste in a local repo path (e.g. `.` or
`/home/you/some-project`), and hit Analyze.

## Deploying to GCP

The Go binary is a single static executable — it runs as-is on **Cloud
Run** (`gcloud run deploy --source .`) with `DATABASE_URL` pointing at a
**Cloud SQL for Postgres** instance via the Cloud SQL Auth Proxy sidecar.
The frontend builds to static files (`npm run build`) deployable to a
Cloud Storage bucket + Cloud CDN, or bundled behind the same Cloud Run
service.

## Project layout

```
cmd/server.go              entrypoint, wires DB + GraphQL handler
internal/analyzer/         pure logic: walks the repo, counts lines, finds hotspots
internal/db/               Postgres schema + caching
internal/graphql/          the GraphQL HTTP handler
web/                       React/TypeScript frontend
migrations/                explicit SQL migration (also auto-applied on boot)
```
