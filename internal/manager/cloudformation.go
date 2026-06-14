package manager

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// CloudFormation extracts runtime/engine versions from CloudFormation
// templates (both YAML and JSON). It looks at `Resources.*.Properties` for a
// fixed set of well-known type/attribute pairs. Long-form intrinsic functions
// (`Ref: X`, `Fn::Sub: ...`) decode as maps so they're skipped by the scalar
// check below. Short-form ones (`!Ref X`) are indistinguishable from a plain
// string after yaml.v3 strips the tag and will leak through — accept that
// limitation; the user can switch to long-form if it matters.
type CloudFormation struct{}

func (CloudFormation) Name() string { return "cloudformation" }

func (CloudFormation) Match(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if !(strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".template")) {
		return false
	}
	// Skip .github/workflows; the GitHub Actions manager owns those.
	return !underGithubWorkflows(path)
}

// cfnResource is the internal shape Extract works against. Properties keys
// map to the raw scalar text of each property (preserves "1.30" vs "1.3"
// for YAML unquoted floats and large ints via json.Number). Non-scalar
// values (intrinsic-function maps, lists) are absent from the map.
type cfnResource struct {
	Type       string
	Properties map[string]string
}

type cfnDoc struct {
	Resources map[string]cfnResource
}

// Decode shapes that preserve raw scalar text.
type cfnYamlDoc struct {
	AWSTemplateFormatVersion string `yaml:"AWSTemplateFormatVersion"`
	Resources                map[string]struct {
		Type       string    `yaml:"Type"`
		Properties yaml.Node `yaml:"Properties"`
	} `yaml:"Resources"`
}

type cfnJSONDoc struct {
	AWSTemplateFormatVersion string `json:"AWSTemplateFormatVersion"`
	Resources                map[string]struct {
		Type       string                 `json:"Type"`
		Properties map[string]interface{} `json:"Properties"`
	} `json:"Resources"`
}

// cfnRule maps a CFN resource Type to (display TYPE, Property name).
type cfnRule struct {
	Type string // TYPE column in our output
	Prop string // Properties key to read
}

var cfnRules = map[string]cfnRule{
	"AWS::Lambda::Function":              {Type: "Lambda", Prop: "Runtime"},
	"AWS::Serverless::Function":          {Type: "Lambda", Prop: "Runtime"}, // SAM
	"AWS::RDS::DBInstance":               {Type: "RDS", Prop: "EngineVersion"},
	"AWS::RDS::DBCluster":                {Type: "Aurora", Prop: "EngineVersion"},
	"AWS::ElastiCache::CacheCluster":     {Type: "ElastiCache", Prop: "EngineVersion"},
	"AWS::ElastiCache::ReplicationGroup": {Type: "ElastiCache", Prop: "EngineVersion"},
	"AWS::OpenSearchService::Domain":     {Type: "OpenSearch", Prop: "EngineVersion"},
	"AWS::Elasticsearch::Domain":         {Type: "OpenSearch", Prop: "ElasticsearchVersion"},
	"AWS::EKS::Cluster":                  {Type: "EKS", Prop: "Version"},
	"AWS::EKS::Nodegroup":                {Type: "EKS Node Group", Prop: "ReleaseVersion"},
}

func (CloudFormation) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	doc, err := decodeCFN(file.Path, data)
	if err != nil || doc.Resources == nil {
		return nil, nil
	}
	// Walk every Resources entry and emit only when its Type matches our
	// cfnRules table (all keys are AWS::... so non-CFN documents that
	// happen to have a `Resources:` key never produce false positives).
	var entries []entry.Entry
	dir := filepath.Dir(file.Path)
	base := filepath.Base(file.Path)
	for name, res := range doc.Resources {
		rule, ok := cfnRules[res.Type]
		if !ok {
			continue
		}
		v, ok := res.Properties[rule.Prop]
		if !ok || v == "" {
			continue
		}
		entries = append(entries, entry.Entry{
			Type:    rule.Type,
			Path:    dir,
			Name:    base + ":" + name,
			Version: v,
		})
	}
	return entries, nil
}

func decodeCFN(path string, data []byte) (*cfnDoc, error) {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".json") {
		return decodeCFNJSON(data)
	}
	return decodeCFNYAML(data)
}

func decodeCFNYAML(data []byte) (*cfnDoc, error) {
	var y cfnYamlDoc
	if err := yaml.Unmarshal(data, &y); err != nil {
		// Not parseable as YAML (short-form `!Ref` tags, Helm `{{ }}`
		// templates, or plain non-CFN files). Extract swallows the error and
		// emits nothing, so there's no need to distinguish CFN from non-CFN
		// here.
		return nil, err
	}
	out := &cfnDoc{Resources: make(map[string]cfnResource, len(y.Resources))}
	for name, r := range y.Resources {
		res := cfnResource{Type: r.Type, Properties: map[string]string{}}
		// r.Properties is a MappingNode whose Content alternates key, value.
		if r.Properties.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(r.Properties.Content); i += 2 {
				k := r.Properties.Content[i].Value
				v := r.Properties.Content[i+1]
				if s, ok := scalarFromYAMLNode(v); ok {
					res.Properties[k] = s
				}
			}
		}
		out.Resources[name] = res
	}
	return out, nil
}

func decodeCFNJSON(data []byte) (*cfnDoc, error) {
	var j cfnJSONDoc
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // keep "18.0" as json.Number rather than losing it to float64.
	if err := dec.Decode(&j); err != nil {
		return nil, err
	}
	out := &cfnDoc{Resources: make(map[string]cfnResource, len(j.Resources))}
	for name, r := range j.Resources {
		res := cfnResource{Type: r.Type, Properties: map[string]string{}}
		for k, raw := range r.Properties {
			if v, ok := scalarFromJSON(raw); ok {
				res.Properties[k] = v
			}
		}
		out.Resources[name] = res
	}
	return out, nil
}

// scalarFromYAMLNode returns the raw text of a scalar YAML node. Non-scalars
// (intrinsic function maps, sequences) are rejected so we don't surface
// "{map}". Using n.Value (rather than letting yaml.v3 decode to float64)
// preserves trailing zeros — `Version: 1.30` stays "1.30".
func scalarFromYAMLNode(n *yaml.Node) (string, bool) {
	if n == nil || n.Kind != yaml.ScalarNode {
		return "", false
	}
	return strings.TrimSpace(n.Value), true
}

// scalarFromJSON returns the string form of a JSON property value when it's
// a plain literal. Intrinsic-function values decoded as maps are rejected.
func scalarFromJSON(v interface{}) (string, bool) {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x), true
	case json.Number:
		return string(x), true
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	}
	return "", false
}
