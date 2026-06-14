package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// Terraform scans each directory that contains *.tf files as an independent
// module and emits required_version, required_providers, and module_calls
// entries via terraform-config-inspect.
type Terraform struct{}

func (Terraform) Name() string { return "terraform" }

// Match claims every *.tf file; GroupByDir then collapses them to their module
// directory so Extract sees the whole module at once.
func (Terraform) Match(path string) bool { return strings.HasSuffix(filepath.Base(path), ".tf") }

func (Terraform) GroupByDir() bool { return true }

func (Terraform) Extract(file File) ([]entry.Entry, error) {
	// A directory we can't even read is a genuine I/O failure worth surfacing;
	// LoadModule itself tolerates malformed individual *.tf files (matching
	// TerraformResources), so its per-file diagnostics stay intentionally
	// dropped.
	if _, err := os.ReadDir(file.Path); err != nil {
		return nil, fmt.Errorf("reading terraform module %s: %w", file.Path, err)
	}
	mod, _ := tfconfig.LoadModule(file.Path)
	if mod == nil {
		return nil, nil
	}
	var entries []entry.Entry
	for _, v := range mod.RequiredCore {
		entries = append(entries, entry.Entry{
			Type:    "Terraform",
			Path:    file.Path,
			Name:    "required_version",
			Version: v,
		})
	}
	providerNames := make([]string, 0, len(mod.RequiredProviders))
	for n := range mod.RequiredProviders {
		providerNames = append(providerNames, n)
	}
	sort.Strings(providerNames)
	for _, name := range providerNames {
		p := mod.RequiredProviders[name]
		for _, c := range p.VersionConstraints {
			entries = append(entries, entry.Entry{
				Type:    "Terraform Provider",
				Path:    file.Path,
				Name:    name,
				Version: c,
			})
		}
	}
	moduleNames := make([]string, 0, len(mod.ModuleCalls))
	for n := range mod.ModuleCalls {
		moduleNames = append(moduleNames, n)
	}
	sort.Strings(moduleNames)
	for _, name := range moduleNames {
		m := mod.ModuleCalls[name]
		if m.Version == "" {
			continue
		}
		entries = append(entries, entry.Entry{
			Type:    "Terraform Module",
			Path:    file.Path,
			Name:    name,
			Version: m.Version,
		})
	}
	return entries, nil
}
