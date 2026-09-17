// Package graphql implements a small, dependency-free GraphQL endpoint.
//
// This is intentionally hand-rolled rather than generated (e.g. via
// gqlgen) so the request/response contract is fully transparent: we parse
// a tiny subset of GraphQL query syntax (single query or mutation, a field
// name, and a flat argument list) which is all this API needs. A real
// production service would likely use gqlgen or graphql-go for a fuller
// spec, but this keeps the wire format identical to GraphQL while staying
// self-contained.
package graphql

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"archaeologist/internal/analyzer"
	"archaeologist/internal/db"
)

type Server struct {
	Store *db.Store
}

type gqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type gqlResponse struct {
	Data   interface{} `json:"data,omitempty"`
	Errors []gqlError  `json:"errors,omitempty"`
}

type gqlError struct {
	Message string `json:"message"`
}

// operationRe extracts the operation kind, field name, and the single
// string argument's variable name from queries like:
//
//	query { analyzeRepo(path: $path) { totalFiles totalLines } }
//	mutation { analyzeRepo(path: $path) { ... } }
//	query { history(limit: $limit) }
var operationRe = regexp.MustCompile(`(query|mutation)\s*{\s*(\w+)\s*\(([^)]*)\)`)
var noArgQueryRe = regexp.MustCompile(`(query|mutation)\s*{\s*(\w+)\s*[{}]?`)

func (s *Server) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}

		var req gqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, "invalid JSON body: "+err.Error())
			return
		}

		field, args, ok := parseOperation(req.Query)
		if !ok {
			writeErr(w, "could not parse operation; expected a single field call")
			return
		}
		resolved := resolveArgs(args, req.Variables)

		switch field {
		case "analyzeRepo":
			s.resolveAnalyzeRepo(w, resolved)
		case "history":
			s.resolveHistory(w, resolved)
		default:
			writeErr(w, fmt.Sprintf("unknown field %q", field))
		}
	}
}

func parseOperation(query string) (field string, args map[string]string, ok bool) {
	query = strings.TrimSpace(query)
	if m := operationRe.FindStringSubmatch(query); m != nil {
		field = m[2]
		args = map[string]string{}
		for _, pair := range strings.Split(m[3], ",") {
			kv := strings.SplitN(pair, ":", 2)
			if len(kv) != 2 {
				continue
			}
			args[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
		return field, args, true
	}
	if m := noArgQueryRe.FindStringSubmatch(query); m != nil {
		return m[2], map[string]string{}, true
	}
	return "", nil, false
}

// resolveArgs turns "$path" -> variables["path"], or strips quotes from a
// literal like "\"/foo\"".
func resolveArgs(args map[string]string, vars map[string]interface{}) map[string]string {
	out := map[string]string{}
	for k, v := range args {
		if strings.HasPrefix(v, "$") {
			if val, ok := vars[strings.TrimPrefix(v, "$")]; ok {
				out[k] = fmt.Sprintf("%v", val)
			}
			continue
		}
		out[k] = strings.Trim(v, `"`)
	}
	return out
}

func (s *Server) resolveAnalyzeRepo(w http.ResponseWriter, args map[string]string) {
	path := args["path"]
	if path == "" {
		writeErr(w, "missing required argument: path")
		return
	}
	forceFresh := args["forceFresh"] == "true"

	if !forceFresh {
		if cached, fresh, err := s.Store.LatestReport(path, 10*time.Minute); err == nil && cached != nil && fresh {
			writeData(w, "analyzeRepo", cached)
			return
		}
	}

	report, err := analyzer.Analyze(path)
	if err != nil {
		writeErr(w, "analysis failed: "+err.Error())
		return
	}
	if _, err := s.Store.SaveReport(report); err != nil {
		// Non-fatal: still return the freshly computed report.
		fmt.Println("warning: failed to cache report:", err)
	}
	writeData(w, "analyzeRepo", report)
}

func (s *Server) resolveHistory(w http.ResponseWriter, args map[string]string) {
	limit := 20
	if v, err := strconv.Atoi(args["limit"]); err == nil && v > 0 {
		limit = v
	}
	paths, err := s.Store.History(limit)
	if err != nil {
		writeErr(w, "history lookup failed: "+err.Error())
		return
	}
	writeData(w, "history", paths)
}

func writeData(w http.ResponseWriter, field string, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gqlResponse{Data: map[string]interface{}{field: payload}})
}

func writeErr(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gqlResponse{Errors: []gqlError{{Message: msg}}})
}
