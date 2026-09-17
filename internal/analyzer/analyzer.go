// Package analyzer walks a git repository on disk and produces a structural
// report: language breakdown by lines of code, dependency manifests found,
// and a lightweight "hotspot" signal based on file size + git churn.
package analyzer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// LanguageStat aggregates line counts for a single detected language.
type LanguageStat struct {
	Language string `json:"language"`
	Files    int    `json:"files"`
	Lines    int    `json:"lines"`
}

// DependencyFile is a manifest we recognized (go.mod, package.json, etc).
type DependencyFile struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// Hotspot is a file that is both large and frequently changed - usually a
// good place to start understanding an unfamiliar codebase.
type Hotspot struct {
	Path        string `json:"path"`
	Lines       int    `json:"lines"`
	CommitCount int    `json:"commitCount"`
}

// Report is the full result of analyzing one repository.
type Report struct {
	RepoPath     string           `json:"repoPath"`
	TotalFiles   int              `json:"totalFiles"`
	TotalLines   int              `json:"totalLines"`
	Languages    []LanguageStat   `json:"languages"`
	Dependencies []DependencyFile `json:"dependencies"`
	Hotspots     []Hotspot        `json:"hotspots"`
}

var extToLang = map[string]string{
	".go":   "Go",
	".ts":   "TypeScript",
	".tsx":  "TypeScript",
	".js":   "JavaScript",
	".jsx":  "JavaScript",
	".py":   "Python",
	".java": "Java",
	".rb":   "Ruby",
	".rs":   "Rust",
	".c":    "C",
	".h":    "C",
	".cpp":  "C++",
	".cs":   "C#",
	".sql":  "SQL",
	".sh":   "Shell",
	".yml":  "YAML",
	".yaml": "YAML",
	".md":   "Markdown",
}

var dependencyFiles = map[string]string{
	"go.mod":           "Go modules",
	"package.json":     "npm/yarn",
	"requirements.txt": "pip",
	"Pipfile":          "pipenv",
	"Cargo.toml":       "cargo",
	"Gemfile":          "bundler",
	"pom.xml":          "Maven",
	"build.gradle":     "Gradle",
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, ".next": true,
}

// Analyze walks repoPath and returns a Report. It does not shell out for
// anything except `git log` (used for the hotspot heuristic); if git isn't
// available or the path isn't a repo, hotspots are simply left empty.
func Analyze(repoPath string) (*Report, error) {
	info, err := os.Stat(repoPath)
	if err != nil {
		return nil, fmt.Errorf("stat repo path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", repoPath)
	}

	langStats := map[string]*LanguageStat{}
	var deps []DependencyFile
	fileLines := map[string]int{}

	err = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries rather than failing the whole walk
		}
		name := d.Name()
		if d.IsDir() {
			if skipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if kind, ok := dependencyFiles[name]; ok {
			rel, _ := filepath.Rel(repoPath, path)
			deps = append(deps, DependencyFile{Path: rel, Kind: kind})
		}

		ext := strings.ToLower(filepath.Ext(name))
		lang, ok := extToLang[ext]
		if !ok {
			return nil
		}
		lines, err := countLines(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(repoPath, path)
		fileLines[rel] = lines

		stat, ok := langStats[lang]
		if !ok {
			stat = &LanguageStat{Language: lang}
			langStats[lang] = stat
		}
		stat.Files++
		stat.Lines += lines
		return nil
	})
	if err != nil {
		return nil, err
	}

	report := &Report{RepoPath: repoPath, Dependencies: deps}
	for _, s := range langStats {
		report.Languages = append(report.Languages, *s)
		report.TotalFiles += s.Files
		report.TotalLines += s.Lines
	}
	sort.Slice(report.Languages, func(i, j int) bool {
		return report.Languages[i].Lines > report.Languages[j].Lines
	})

	report.Hotspots = computeHotspots(repoPath, fileLines)
	return report, nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	n := 0
	for scanner.Scan() {
		n++
	}
	return n, nil
}

// computeHotspots ranks files by lines * commit_count, using `git log
// --follow --format= --name-only` to get per-file commit counts. If the
// directory isn't a git repo this returns nil without error.
func computeHotspots(repoPath string, fileLines map[string]int) []Hotspot {
	cmd := exec.Command("git", "-C", repoPath, "log", "--pretty=format:", "--name-only")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	counts := map[string]int{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		counts[line]++
	}

	var hotspots []Hotspot
	for path, lines := range fileLines {
		if c, ok := counts[path]; ok && c > 0 {
			hotspots = append(hotspots, Hotspot{Path: path, Lines: lines, CommitCount: c})
		}
	}
	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].Lines*hotspots[i].CommitCount > hotspots[j].Lines*hotspots[j].CommitCount
	})
	if len(hotspots) > 15 {
		hotspots = hotspots[:15]
	}
	return hotspots
}
