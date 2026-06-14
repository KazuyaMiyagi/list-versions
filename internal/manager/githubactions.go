package manager

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// GithubActions reads workflow files under .github/workflows and extracts
// two kinds of version declarations:
//
//  1. Action references: `uses: actions/checkout@v4` -> Version="actions/checkout@v4"
//     If a same-line comment is present (the Dependabot/Renovate convention of
//     pinning to a SHA with a human-readable tag, e.g. `@SHA # v4.2.2`), it is
//     appended as ` # v4.2.2` so the readable tag stays visible.
//  2. setup-X actions:   `with: { node-version: 20 }` -> TYPE="Node.js", Version="20"
type GithubActions struct{}

func (GithubActions) Name() string { return "github-actions" }

func (GithubActions) Match(path string) bool {
	name := filepath.Base(path)
	if !(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
		return false
	}
	// Only files directly under a `.github/workflows/` segment.
	return underGithubWorkflows(path)
}

func (GithubActions) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	// Empty document or a workflow that simply has no `jobs:` key (e.g.
	// reusable composite action definition) is benign — silently skip.
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, nil
	}
	doc := root.Content[0]
	jobs := findMapChild(doc, "jobs")
	if jobs == nil {
		return nil, nil
	}

	base := filepath.Base(file.Path)
	dir := filepath.Dir(file.Path)
	seen := map[string]bool{}
	var entries []entry.Entry

	// emit dedupes by `(type, dedupKey)` so that two `uses: foo/bar@v4` lines
	// with different trailing comments still collapse to one row.
	emit := func(typeName, dedupKey, version string) {
		if version == "" {
			return
		}
		key := typeName + "|" + dedupKey
		if seen[key] {
			return
		}
		seen[key] = true
		entries = append(entries, entry.Entry{
			Type:    typeName,
			Path:    dir,
			Name:    base,
			Version: version,
		})
	}

	workflowEnv := collectEnv(findMapChild(doc, "env"))

	// jobs is a mapping of job_name -> job
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		job := jobs.Content[i+1]
		jobEnv := mergeEnv(workflowEnv, collectEnv(findMapChild(job, "env")))
		matrix := collectMatrix(findMapChild(findMapChild(job, "strategy"), "matrix"))
		steps := findMapChild(job, "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range steps.Content {
			stepEnv := mergeEnv(jobEnv, collectEnv(findMapChild(step, "env")))
			var usesValue string
			if uses := findMapChild(step, "uses"); uses != nil && uses.Kind == yaml.ScalarNode {
				usesValue = uses.Value
				if t, v := classifyUses(uses.Value, stripCommentPrefix(uses.LineComment)); t != "" {
					emit(t, uses.Value, v)
				}
			}
			if with := findMapChild(step, "with"); with != nil && with.Kind == yaml.MappingNode {
				for j := 0; j+1 < len(with.Content); j += 2 {
					k := with.Content[j].Value
					raw := strings.TrimSpace(with.Content[j+1].Value)
					if raw == "" {
						continue
					}
					resolved := resolveEnvExpr(raw, stepEnv)
					for _, v := range expandMatrixExpr(resolved, matrix) {
						switch {
						case strings.HasSuffix(k, "-version-file"):
							tool := strings.TrimSuffix(k, "-version-file")
							emit(typeFromTool(tool), k+"="+v, v)
						case strings.HasSuffix(k, "-version"):
							tool := strings.TrimSuffix(k, "-version")
							emit(typeFromTool(tool), v, v)
						case k == "version":
							if tool := detectSetupTool(usesValue); tool != "" {
								emit(typeFromTool(tool), v, v)
							}
						}
					}
				}
			}
		}
	}
	return entries, nil
}

// envExprRe matches `${{ env.X }}` (with arbitrary surrounding whitespace and
// dashes/digits in the name). Anything more complex (function calls, indexing,
// other contexts) is left untouched.
var envExprRe = regexp.MustCompile(`\$\{\{\s*env\.([\w-]+)\s*\}\}`)

// resolveEnvExpr replaces every `${{ env.X }}` in s with env[X]. Unknown
// env keys are kept as-is so the reader can still tell something needs
// looking up.
func resolveEnvExpr(s string, env map[string]string) string {
	if !strings.Contains(s, "${{") {
		return s
	}
	return envExprRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := envExprRe.FindStringSubmatch(match)
		if v, ok := env[sub[1]]; ok {
			return v
		}
		return match
	})
}

// collectEnv pulls scalar key/value pairs out of a workflow `env:` node.
// Non-scalar values (e.g. expressions resolved at runtime) are stored as-is so
// they can still chain through resolveEnvExpr in callers.
func collectEnv(n *yaml.Node) map[string]string {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	out := map[string]string{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i].Value
		v := n.Content[i+1]
		if v.Kind == yaml.ScalarNode {
			out[k] = v.Value
		}
	}
	return out
}

// matrixExprRe matches `${{ matrix.X }}` references with a single dotted key.
var matrixExprRe = regexp.MustCompile(`\$\{\{\s*matrix\.([\w-]+)\s*\}\}`)

// collectMatrix pulls `strategy.matrix.<key>: [v1, v2, ...]` definitions into
// a map. `include`/`exclude` are skipped because they describe how matrix
// combinations are filtered rather than which values exist for a single key.
// Scalar values are wrapped as a single-element slice.
func collectMatrix(n *yaml.Node) map[string][]string {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	out := map[string][]string{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i].Value
		if k == "include" || k == "exclude" {
			continue
		}
		v := n.Content[i+1]
		switch v.Kind {
		case yaml.SequenceNode:
			var vals []string
			for _, item := range v.Content {
				if item.Kind == yaml.ScalarNode {
					vals = append(vals, item.Value)
				}
			}
			if len(vals) > 0 {
				out[k] = vals
			}
		case yaml.ScalarNode:
			if v.Value != "" {
				out[k] = []string{v.Value}
			}
		}
	}
	return out
}

// expandMatrixExpr expands every `${{ matrix.X }}` reference whose key has
// known values, taking the Cartesian product when several distinct keys are
// referenced in the same string. Unknown keys are left as-is so the reader
// can still tell where the value came from.
func expandMatrixExpr(s string, matrix map[string][]string) []string {
	if len(matrix) == 0 || !strings.Contains(s, "${{") {
		return []string{s}
	}
	matches := matrixExprRe.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return []string{s}
	}
	keysSeen := map[string]bool{}
	var keys []string
	for _, m := range matches {
		k := m[1]
		if _, ok := matrix[k]; !ok {
			continue
		}
		if keysSeen[k] {
			continue
		}
		keysSeen[k] = true
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return []string{s}
	}
	results := []string{s}
	for _, k := range keys {
		altRe := matrixKeyRe(k)
		var next []string
		for _, cur := range results {
			for _, v := range matrix[k] {
				next = append(next, altRe.ReplaceAllString(cur, v))
			}
		}
		results = next
	}
	return results
}

// matrixKeyRe returns (and caches) the regexp matching `${{ matrix.<key> }}`
// for a specific key. The cache keeps expandMatrixExpr from recompiling the
// same pattern for every step and workflow file in a large scan.
var matrixKeyReCache sync.Map // string -> *regexp.Regexp

func matrixKeyRe(key string) *regexp.Regexp {
	if v, ok := matrixKeyReCache.Load(key); ok {
		return v.(*regexp.Regexp)
	}
	re := regexp.MustCompile(`\$\{\{\s*matrix\.` + regexp.QuoteMeta(key) + `\s*\}\}`)
	matrixKeyReCache.Store(key, re)
	return re
}

// mergeEnv returns a new map containing `parent` overlaid with `child` so the
// inner scope wins. Either argument may be nil.
func mergeEnv(parent, child map[string]string) map[string]string {
	if len(parent) == 0 && len(child) == 0 {
		return nil
	}
	out := make(map[string]string, len(parent)+len(child))
	for k, v := range parent {
		out[k] = v
	}
	for k, v := range child {
		out[k] = v
	}
	return out
}

// detectSetupTool guesses the tool name from a setup-style action reference.
// Examples:
//
//	actions/setup-node@v4       -> "node"
//	actions/setup-python@v5     -> "python"
//	ruby/setup-ruby@v1          -> "ruby"
//	oven-sh/setup-bun@v1        -> "bun"
//	pnpm/action-setup@v3        -> "pnpm"
//	helm/kind-action@v1.10.0    -> ""   (no setup pattern matched)
func detectSetupTool(uses string) string {
	ref := uses
	if at := strings.Index(ref, "@"); at >= 0 {
		ref = ref[:at]
	}
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return ""
	}
	owner, repo := parts[0], parts[1]
	switch {
	case repo == "action-setup":
		return owner
	case strings.HasPrefix(repo, "setup-"):
		return strings.TrimPrefix(repo, "setup-")
	case strings.HasSuffix(repo, "-setup"):
		return strings.TrimSuffix(repo, "-setup")
	}
	return ""
}

func findMapChild(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// stripCommentPrefix turns a yaml.Node line comment like "# v4.2.2" into
// "v4.2.2". Returns empty when there is no comment.
func stripCommentPrefix(c string) string {
	c = strings.TrimSpace(c)
	c = strings.TrimPrefix(c, "#")
	return strings.TrimSpace(c)
}

// classifyUses turns a `uses:` value into (type, version). Local actions
// (./...) and Docker actions (docker://...) are dropped because we have no
// version to report from the reference alone. If `comment` is non-empty it is
// appended as " # comment" to preserve the human-readable tag that
// dependency-management bots leave next to a SHA pin.
func classifyUses(uses, comment string) (string, string) {
	if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "docker://") {
		return "", ""
	}
	if !strings.Contains(uses, "@") {
		return "", ""
	}
	version := uses
	if comment != "" {
		version += " # " + comment
	}
	return "GitHub Actions", version
}
