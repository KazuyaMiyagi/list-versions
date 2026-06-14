package manager

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// Helm reads Chart.yaml files to surface the chart's `appVersion` and own
// `version`, and the sibling `values.yaml` for a top-level `image.tag` if it's
// a literal scalar. Anything more elaborate (sub-charts, multi-image values)
// is left to the user.
type Helm struct{}

func (Helm) Name() string { return "helm" }

func (Helm) Match(path string) bool { return filepath.Base(path) == "Chart.yaml" }

type chartYaml struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	AppVersion string `yaml:"appVersion"`
}

type valuesYaml struct {
	Image struct {
		Repository string `yaml:"repository"`
		Tag        string `yaml:"tag"`
	} `yaml:"image"`
}

func (Helm) Extract(file File) ([]entry.Entry, error) {
	dir := filepath.Dir(file.Path)
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var chart chartYaml
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return nil, err
	}

	var entries []entry.Entry
	if chart.AppVersion != "" {
		entries = append(entries, entry.Entry{
			Type:    "Helm Chart",
			Path:    dir,
			Name:    "Chart.yaml:appVersion",
			Version: chart.AppVersion,
		})
	}
	if chart.Version != "" {
		entries = append(entries, entry.Entry{
			Type:    "Helm Chart",
			Path:    dir,
			Name:    "Chart.yaml:version",
			Version: chart.Version,
		})
	}

	// Sibling values.yaml `image.tag` if it's a plain literal.
	valuesPath := filepath.Join(dir, "values.yaml")
	if data, err := os.ReadFile(valuesPath); err == nil {
		var vals valuesYaml
		if err := yaml.Unmarshal(data, &vals); err == nil && vals.Image.Tag != "" {
			ver := vals.Image.Tag
			if vals.Image.Repository != "" {
				ver = vals.Image.Repository + ":" + vals.Image.Tag
			}
			entries = append(entries, entry.Entry{
				Type:    "Helm Chart",
				Path:    dir,
				Name:    "values.yaml:image.tag",
				Version: ver,
			})
		}
	}
	return entries, nil
}
