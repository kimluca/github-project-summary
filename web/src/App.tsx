import { useState } from "react";
import { analyzeRepo, fetchHistory, Report } from "./graphqlClient";

export default function App() {
  const [path, setPath] = useState("");
  const [report, setReport] = useState<Report | null>(null);
  const [history, setHistory] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function runAnalysis() {
    if (!path.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const r = await analyzeRepo(path.trim());
      setReport(r);
      setHistory(await fetchHistory());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{ maxWidth: 720, margin: "40px auto", fontFamily: "system-ui, sans-serif" }}>
      <h1>🏺 Codebase Archaeologist</h1>
      <p style={{ color: "#555" }}>
        Point this at a local path to a git repo. It walks the tree, tallies lines of code
        per language, spots dependency manifests, and (if it's a git repo) ranks the
        files most worth reading first.
      </p>

      <div style={{ display: "flex", gap: 8, margin: "16px 0" }}>
        <input
          value={path}
          onChange={(e) => setPath(e.target.value)}
          placeholder="/path/to/repo"
          style={{ flex: 1, padding: 8 }}
          onKeyDown={(e) => e.key === "Enter" && runAnalysis()}
        />
        <button onClick={runAnalysis} disabled={loading}>
          {loading ? "Digging..." : "Analyze"}
        </button>
      </div>

      {error && <p style={{ color: "crimson" }}>{error}</p>}

      {report && (
        <div>
          <h2>{report.repoPath}</h2>
          <p>
            {report.totalFiles} files · {report.totalLines.toLocaleString()} lines
          </p>

          <h3>Languages</h3>
          <table width="100%" cellPadding={4}>
            <thead>
              <tr>
                <th align="left">Language</th>
                <th align="right">Files</th>
                <th align="right">Lines</th>
              </tr>
            </thead>
            <tbody>
              {report.languages.map((l) => (
                <tr key={l.language}>
                  <td>{l.language}</td>
                  <td align="right">{l.files}</td>
                  <td align="right">{l.lines.toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>

          <h3>Dependency manifests</h3>
          <ul>
            {report.dependencies.map((d) => (
              <li key={d.path}>
                {d.path} <em>({d.kind})</em>
              </li>
            ))}
          </ul>

          {report.hotspots && report.hotspots.length > 0 && (
            <>
              <h3>Hotspots (large + frequently changed)</h3>
              <ol>
                {report.hotspots.map((h) => (
                  <li key={h.path}>
                    {h.path} — {h.lines} lines, {h.commitCount} commits
                  </li>
                ))}
              </ol>
            </>
          )}
        </div>
      )}

      {history.length > 0 && (
        <div style={{ marginTop: 32, color: "#777" }}>
          <h4>Previously analyzed</h4>
          <ul>
            {history.map((h) => (
              <li key={h}>{h}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
