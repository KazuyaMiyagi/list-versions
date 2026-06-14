package manager

import (
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// Serverless reads Serverless Framework manifests (`serverless.yml` /
// `serverless.yaml`). It surfaces the `provider.runtime` default and each
// `functions.<id>.runtime` override.
type Serverless struct{}

func (Serverless) Name() string { return "serverless" }

func (Serverless) Match(path string) bool {
	name := filepath.Base(path)
	return name == "serverless.yml" || name == "serverless.yaml"
}

type serverlessDoc struct {
	Provider struct {
		Runtime string `yaml:"runtime"`
	} `yaml:"provider"`
	Functions map[string]struct {
		Runtime string `yaml:"runtime"`
	} `yaml:"functions"`
}

func (Serverless) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc serverlessDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	dir := filepath.Dir(file.Path)
	base := filepath.Base(file.Path)
	var entries []entry.Entry
	if r := doc.Provider.Runtime; r != "" {
		entries = append(entries, entry.Entry{
			Type:    "Lambda",
			Path:    dir,
			Name:    base + ":provider",
			Version: r,
		})
	}
	for name, fn := range doc.Functions {
		if fn.Runtime == "" {
			continue
		}
		entries = append(entries, entry.Entry{
			Type:    "Lambda",
			Path:    dir,
			Name:    base + ":functions." + name,
			Version: fn.Runtime,
		})
	}
	return entries, nil
}
