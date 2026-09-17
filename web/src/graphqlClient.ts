// Minimal typed GraphQL client - no codegen, no Apollo, just fetch + types.
// Matches the hand-rolled server in internal/graphql/graphql.go.

export interface LanguageStat {
  language: string;
  files: number;
  lines: number;
}

export interface DependencyFile {
  path: string;
  kind: string;
}

export interface Hotspot {
  path: string;
  lines: number;
  commitCount: number;
}

export interface Report {
  repoPath: string;
  totalFiles: number;
  totalLines: number;
  languages: LanguageStat[];
  dependencies: DependencyFile[];
  hotspots: Hotspot[] | null;
}

interface GraphQLResponse<T> {
  data?: T;
  errors?: { message: string }[];
}

async function gql<T>(query: string, variables: Record<string, unknown>): Promise<T> {
  const res = await fetch("/graphql", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query, variables }),
  });
  const json: GraphQLResponse<T> = await res.json();
  if (json.errors?.length) {
    throw new Error(json.errors.map((e) => e.message).join("; "));
  }
  if (!json.data) {
    throw new Error("empty response from server");
  }
  return json.data;
}

export async function analyzeRepo(path: string, forceFresh = false): Promise<Report> {
  const data = await gql<{ analyzeRepo: Report }>(
    `query { analyzeRepo(path: $path, forceFresh: $forceFresh) { totalFiles totalLines } }`,
    { path, forceFresh }
  );
  return data.analyzeRepo;
}

export async function fetchHistory(limit = 20): Promise<string[]> {
  const data = await gql<{ history: string[] }>(`query { history(limit: $limit) }`, { limit });
  return data.history;
}
