package manager

import "strings"

// fileTypeMap maps a file basename to the TYPE label.
// Based on the labels used in the existing bash list-version-files
// and the SW_UPDATE 表 vocabulary referenced in the design memo.
var fileTypeMap = map[string]string{
	".python-version":            "Python",
	".ruby-version":              "Ruby",
	".rvmrc":                     "Ruby",
	".node-version":              "Node.js",
	".nvmrc":                     "Node.js",
	".terraform-version":         "Terraform",
	".openapi-generator-version": "OpenAPI Generator",
	".go-version":                "Go",
	".java-version":              "Java",
	".elixir-version":            "Elixir",
	".erlang-version":            "Erlang",
	".crystal-version":           "Crystal",
	".lua-version":               "Lua",
	".perl-version":              "Perl",
	".php-version":               "PHP",
	".sbt-version":               "sbt",
	".scala-version":             "Scala",
	".swift-version":             "Swift",
	".bun-version":               "Bun",
}

// toolTypeMap maps an asdf/mise/.tool-versions tool identifier to TYPE.
var toolTypeMap = map[string]string{
	"python":            "Python",
	"ruby":              "Ruby",
	"nodejs":            "Node.js",
	"node":              "Node.js",
	"terraform":         "Terraform",
	"golang":            "Go",
	"go":                "Go",
	"java":              "Java",
	"elixir":            "Elixir",
	"erlang":            "Erlang",
	"crystal":           "Crystal",
	"lua":               "Lua",
	"perl":              "Perl",
	"php":               "PHP",
	"sbt":               "sbt",
	"scala":             "Scala",
	"swift":             "Swift",
	"bun":               "Bun",
	"deno":              "Deno",
	"rust":              "Rust",
	"openapi-generator": "OpenAPI Generator",
	"pnpm":              "pnpm",
	"yarn":              "Yarn",
	"npm":               "npm",
	"dotnet":            ".NET",
}

// typeFromVersionFile derives the TYPE label from the basename of a
// `.X-version` style file. Unknown filenames fall back to the middle
// segment (e.g. ".foo-version" -> "foo").
func typeFromVersionFile(base string) string {
	if t, ok := fileTypeMap[base]; ok {
		return t
	}
	t := strings.TrimPrefix(base, ".")
	t = strings.TrimSuffix(t, "-version")
	// A degenerate name like ".-version" trims to empty; fall back to the raw
	// basename so the Entry never carries an empty TYPE.
	if t == "" {
		return base
	}
	return t
}

// typeFromTool derives TYPE for a tool name listed in `.tool-versions`.
// Unknown tools are returned as-is.
func typeFromTool(tool string) string {
	if t, ok := toolTypeMap[tool]; ok {
		return t
	}
	return tool
}
