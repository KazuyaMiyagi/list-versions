package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// tfEvalCtx exposes the handful of HCL functions we need to fold static
// expressions into literal values. We only register pure-string ones so we
// don't pull in network/filesystem-side-effecting helpers like `file()`.
var tfEvalCtx = &hcl.EvalContext{
	Functions: map[string]function.Function{
		"jsonencode": stdlib.JSONEncodeFunc,
	},
}

// TerraformResources reads *.tf files directly with the HCL parser and pulls
// version-bearing attributes off well-known AWS resources (Lambda runtime,
// RDS / Aurora engine version, ElastiCache, OpenSearch, EKS).
// It complements the existing Terraform Manager, which only surfaces
// `required_version` / providers / modules via terraform-config-inspect.
type TerraformResources struct{}

func (TerraformResources) Name() string { return "terraform-resources" }

// Match claims every *.tf file; GroupByDir then groups them by their containing
// directory so Extract sees the whole Terraform module at once. Cross-file
// relationships like `aws_cloudfront_distribution` in one file referencing
// `aws_lambda_function` in another (the typical Lambda@Edge layout) only
// resolve correctly that way.
func (TerraformResources) Match(path string) bool {
	return strings.HasSuffix(filepath.Base(path), ".tf")
}

func (TerraformResources) GroupByDir() bool { return true }

// Extract parses every *.tf in the module directory, then walks the union of
// their blocks twice: once to collect Lambda@Edge function names from every
// CloudFront distribution, and once to emit entries with the correct TYPE.
//
// Note: because this manager and the Terraform manager are both DirScoped (Data
// nil), each re-reads the module's *.tf files from disk independently — the
// shared read-once cache does not cover Terraform. The cost is one extra parse
// pass per module, accepted to keep both managers self-contained.
func (TerraformResources) Extract(file File) ([]entry.Entry, error) {
	dir := file.Path
	bodies, files, err := parseModuleTfFiles(dir)
	if err != nil {
		// Only a directory read failure (permission denied, dir removed mid-
		// scan) reaches here; malformed individual *.tf files are tolerated
		// inside parseModuleTfFiles. Surface the genuine I/O error so it's not
		// silently mistaken for an empty module.
		return nil, fmt.Errorf("reading terraform module %s: %w", dir, err)
	}
	if len(bodies) == 0 {
		return nil, nil
	}

	edgeFuncs := map[string]bool{}
	for _, body := range bodies {
		for name := range collectEdgeFunctions(body) {
			edgeFuncs[name] = true
		}
	}

	var entries []entry.Entry
	for i, body := range bodies {
		base := files[i]
		for _, block := range body.Blocks {
			if block.Type != "resource" || len(block.Labels) < 2 {
				continue
			}
			entries = append(entries, classifyAWSResource(block, dir, base, edgeFuncs)...)
		}
	}
	return entries, nil
}

// parseModuleTfFiles reads every *.tf file directly inside `dir` (non-
// recursive — Terraform modules are flat by convention) and returns their
// parsed bodies alongside the file basenames, both in lockstep.
func parseModuleTfFiles(dir string) ([]*hclsyntax.Body, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".tf") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	parser := hclparse.NewParser()
	var bodies []*hclsyntax.Body
	var kept []string
	for _, n := range names {
		parsed, diags := parser.ParseHCLFile(filepath.Join(dir, n))
		if diags.HasErrors() {
			continue
		}
		body, ok := parsed.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		bodies = append(bodies, body)
		kept = append(kept, n)
	}
	return bodies, kept, nil
}

// collectEdgeFunctions walks every aws_cloudfront_distribution block in the
// file, dives into `default_cache_behavior` / `ordered_cache_behavior` /
// nested `lambda_function_association` blocks, and reads the `lambda_arn`
// attribute. The attribute is almost always a reference like
// `aws_lambda_function.foo.qualified_arn`; we extract the resource name
// ("foo") so the lambda block can re-label itself.
func collectEdgeFunctions(body *hclsyntax.Body) map[string]bool {
	out := map[string]bool{}
	for _, block := range body.Blocks {
		if block.Type != "resource" || len(block.Labels) < 1 || block.Labels[0] != "aws_cloudfront_distribution" {
			continue
		}
		for _, behavior := range block.Body.Blocks {
			if behavior.Type != "default_cache_behavior" && behavior.Type != "ordered_cache_behavior" {
				continue
			}
			for _, assoc := range behavior.Body.Blocks {
				if assoc.Type != "lambda_function_association" {
					continue
				}
				attr, ok := assoc.Body.Attributes["lambda_arn"]
				if !ok {
					continue
				}
				for _, name := range traversalLambdaName(attr.Expr) {
					out[name] = true
				}
			}
		}
	}
	return out
}

// traversalLambdaName extracts `aws_lambda_function.<name>` references from
// a Terraform expression. Returns nil for literal strings or unrecognised
// expressions — we only label when the reference is unambiguous.
func traversalLambdaName(expr hclsyntax.Expression) []string {
	var out []string
	switch e := expr.(type) {
	case *hclsyntax.ScopeTraversalExpr:
		if n := lambdaNameFromTraversal(e.Traversal); n != "" {
			out = append(out, n)
		}
	case *hclsyntax.RelativeTraversalExpr:
		// e.g. `aws_lambda_function.foo.qualified_arn`. The Source may be an
		// expression with no resource references at all (function calls,
		// literals, etc.); Variables() returns an empty slice in that case
		// so we have to len-check before indexing.
		vars := e.Source.Variables()
		if len(vars) == 0 {
			return out
		}
		if n := lambdaNameFromTraversal(vars[0]); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func lambdaNameFromTraversal(t hcl.Traversal) string {
	if len(t) < 2 {
		return ""
	}
	root, ok := t[0].(hcl.TraverseRoot)
	if !ok || root.Name != "aws_lambda_function" {
		return ""
	}
	attr, ok := t[1].(hcl.TraverseAttr)
	if !ok {
		return ""
	}
	return attr.Name
}

// classifyAWSResource dispatches on the resource type label and returns any
// Entry rows that describe an actual runtime/engine version. Attributes that
// reference variables or locals (i.e. can't be evaluated without context) are
// silently skipped — we only surface literal versions.
func classifyAWSResource(block *hclsyntax.Block, dir, base string, edgeFuncs map[string]bool) []entry.Entry {
	resType := block.Labels[0]
	resName := block.Labels[1]
	label := resType + "." + resName
	name := base + ":" + label

	readStr := func(attr string) string { return readAttrString(block, attr) }
	add := func(typeName, version string) entry.Entry {
		return entry.Entry{Type: typeName, Path: dir, Name: name, Version: version}
	}
	var out []entry.Entry
	maybe := func(typeName, version string) {
		if version == "" {
			return
		}
		out = append(out, add(typeName, version))
	}

	lambdaType := "Lambda"
	if edgeFuncs[resName] {
		lambdaType = "Lambda@Edge"
	}
	switch resType {
	case "aws_lambda_function":
		maybe(lambdaType, readStr("runtime"))
	case "aws_lambda_layer_version":
		for _, v := range readAttrStringList(block, "compatible_runtimes") {
			out = append(out, add("Lambda", v))
		}
	case "aws_db_instance":
		maybe("RDS", joinEngineVersion(readStr("engine"), readStr("engine_version")))
	case "aws_rds_cluster":
		engine := readStr("engine")
		ev := readStr("engine_version")
		if ev == "" {
			break
		}
		typeName := "Aurora"
		if engine != "" && !strings.HasPrefix(engine, "aurora") {
			typeName = "RDS"
		}
		maybe(typeName, joinEngineVersion(engine, ev))
	case "aws_elasticache_cluster", "aws_elasticache_replication_group":
		engine := strings.ToLower(readStr("engine"))
		ev := readStr("engine_version")
		if ev == "" {
			break
		}
		typeName := "ElastiCache"
		switch engine {
		case "redis":
			typeName = "ElastiCache (Redis)"
		case "memcached":
			typeName = "ElastiCache (Memcached)"
		case "valkey":
			typeName = "ElastiCache (Valkey)"
		}
		maybe(typeName, ev)
	case "aws_opensearch_domain", "aws_elasticsearch_domain":
		maybe("OpenSearch", readStr("engine_version"))
	case "aws_eks_cluster":
		maybe("EKS", readStr("version"))
	case "aws_eks_node_group":
		maybe("EKS Node Group", readStr("release_version"))
	case "google_cloud_run_v2_service":
		for _, img := range collectNestedImages(block, []string{"template", "containers"}) {
			maybe("Cloud Run", img)
		}
	case "google_cloud_run_v2_job":
		for _, img := range collectNestedImages(block, []string{"template", "template", "containers"}) {
			maybe("Cloud Run", img)
		}
	case "google_cloud_run_service":
		// Gen1 (deprecated). Path: spec > containers > image.
		for _, img := range collectNestedImages(block, []string{"template", "spec", "containers"}) {
			maybe("Cloud Run", img)
		}
	case "aws_ecs_task_definition":
		for _, img := range readEcsContainerImages(block) {
			maybe("ECS", img)
		}
	case "aws_instance":
		maybe("AMI", readStr("ami"))
	case "aws_launch_template":
		maybe("AMI", readStr("image_id"))
	case "aws_launch_configuration":
		maybe("AMI", readStr("image_id"))
	case "google_compute_instance", "google_compute_instance_template":
		// boot_disk > initialize_params > image
		for _, bd := range block.Body.Blocks {
			if bd.Type != "boot_disk" {
				continue
			}
			for _, ip := range bd.Body.Blocks {
				if ip.Type != "initialize_params" {
					continue
				}
				maybe("Compute Image", readAttrString(ip, "image"))
			}
		}
	case "google_cloudfunctions_function":
		maybe("Cloud Functions", readStr("runtime"))
	case "google_cloudfunctions2_function":
		// build_config.runtime
		for _, nb := range block.Body.Blocks {
			if nb.Type != "build_config" {
				continue
			}
			maybe("Cloud Functions", readAttrString(nb, "runtime"))
		}
	case "google_app_engine_standard_app_version":
		maybe("App Engine", readStr("runtime"))
	case "google_app_engine_flexible_app_version":
		maybe("App Engine", readStr("runtime"))
	case "google_sql_database_instance":
		maybe("Cloud SQL", readStr("database_version"))
	case "aws_apprunner_service":
		// source_configuration > image_repository > image_identifier
		for _, sc := range block.Body.Blocks {
			if sc.Type != "source_configuration" {
				continue
			}
			for _, ir := range sc.Body.Blocks {
				if ir.Type != "image_repository" {
					continue
				}
				maybe("App Runner", readAttrString(ir, "image_identifier"))
			}
		}
	}
	return out
}

// readEcsContainerImages handles aws_ecs_task_definition.container_definitions,
// which is conventionally written as jsonencode([{...}, {...}]) in Terraform.
// The HCL EvalContext folds the call into a literal JSON string we can decode.
func readEcsContainerImages(block *hclsyntax.Block) []string {
	attr, ok := block.Body.Attributes["container_definitions"]
	if !ok {
		return nil
	}
	val, diags := attr.Expr.Value(tfEvalCtx)
	if diags.HasErrors() || val.IsNull() || val.Type() != cty.String {
		return nil
	}
	var defs []map[string]any
	if err := json.Unmarshal([]byte(val.AsString()), &defs); err != nil {
		return nil
	}
	var out []string
	for _, d := range defs {
		if img, ok := d["image"].(string); ok && img != "" {
			out = append(out, img)
		}
	}
	return out
}

// collectNestedImages walks nested HCL blocks along `path` and returns the
// `image` attribute of every leaf block. Used for resources whose container
// images sit one or two `template` blocks deep (Cloud Run v2, jobs, Gen1).
func collectNestedImages(block *hclsyntax.Block, path []string) []string {
	if len(path) == 0 {
		if v := readAttrString(block, "image"); v != "" {
			return []string{v}
		}
		return nil
	}
	head := path[0]
	rest := path[1:]
	var out []string
	for _, nb := range block.Body.Blocks {
		if nb.Type != head {
			continue
		}
		out = append(out, collectNestedImages(nb, rest)...)
	}
	return out
}

// joinEngineVersion combines an engine name with its version for the VERSION
// column. When the version is absent (RDS/Aurora let AWS pick a default) it
// returns "" so callers via maybe() skip the row rather than leaking the bare
// engine name as if it were a version.
func joinEngineVersion(engine, ev string) string {
	if ev == "" {
		return ""
	}
	if engine == "" {
		return ev
	}
	return engine + " " + ev
}

func readAttrString(block *hclsyntax.Block, name string) string {
	attr, ok := block.Body.Attributes[name]
	if !ok {
		return ""
	}
	val, diags := attr.Expr.Value(nil)
	if diags.HasErrors() || val.IsNull() || val.Type() != cty.String {
		return ""
	}
	return val.AsString()
}

func readAttrStringList(block *hclsyntax.Block, name string) []string {
	attr, ok := block.Body.Attributes[name]
	if !ok {
		return nil
	}
	val, diags := attr.Expr.Value(nil)
	if diags.HasErrors() || val.IsNull() {
		return nil
	}
	t := val.Type()
	if !(t.IsListType() || t.IsTupleType() || t.IsSetType()) {
		return nil
	}
	var out []string
	for it := val.ElementIterator(); it.Next(); {
		_, v := it.Element()
		if v.Type() == cty.String && !v.IsNull() {
			out = append(out, v.AsString())
		}
	}
	return out
}
