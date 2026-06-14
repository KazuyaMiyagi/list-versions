package manager

import (
	"path/filepath"
	"testing"
)

func TestCloudFormation_YAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "template.yaml"), `
AWSTemplateFormatVersion: '2010-09-09'
Resources:
  ApiFunction:
    Type: AWS::Lambda::Function
    Properties:
      Runtime: nodejs22.x
      Handler: index.handler
  Database:
    Type: AWS::RDS::DBInstance
    Properties:
      Engine: mysql
      EngineVersion: 8.0.35
  AuroraCluster:
    Type: AWS::RDS::DBCluster
    Properties:
      Engine: aurora-mysql
      EngineVersion: 8.0.mysql_aurora.3.06.0
  Cluster:
    Type: AWS::EKS::Cluster
    Properties:
      Version: "1.29"
  RuntimeFromRef:
    Type: AWS::Lambda::Function
    Properties:
      Runtime:
        Ref: RuntimeParam   # long-form intrinsic -> skipped
  Unrelated:
    Type: AWS::S3::Bucket
    Properties:
      BucketName: my-bucket
`)
	files := matched(t, CloudFormation{}, dir)
	if len(files) != 1 {
		t.Fatalf("detect: %v", files)
	}
	es := collect(t, CloudFormation{}, dir)

	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	want := map[string]string{
		"Lambda": "nodejs22.x",
		"RDS":    "8.0.35",
		"Aurora": "8.0.mysql_aurora.3.06.0",
		"EKS":    "1.29",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestCloudFormation_UnquotedNumericVersion(t *testing.T) {
	dir := t.TempDir()
	// Unquoted scalars decode as numbers; we must preserve the trailing zero.
	writeFile(t, filepath.Join(dir, "template.yaml"), `
AWSTemplateFormatVersion: '2010-09-09'
Resources:
  Cluster:
    Type: AWS::EKS::Cluster
    Properties:
      Version: 1.30
  Db:
    Type: AWS::RDS::DBInstance
    Properties:
      Engine: postgres
      EngineVersion: 18.0
`)
	es := collect(t, CloudFormation{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Type] = e.Version
	}
	if got["EKS"] != "1.30" {
		t.Errorf("EKS Version: got %q, want %q", got["EKS"], "1.30")
	}
	if got["RDS"] != "18.0" {
		t.Errorf("RDS EngineVersion: got %q, want %q", got["RDS"], "18.0")
	}
}

func TestCloudFormation_InvalidYAMLProducesNoEntries(t *testing.T) {
	dir := t.TempDir()
	// A Helm template is not valid YAML before rendering; it must yield no
	// entries and must not surface as a fatal scan error.
	writeFile(t, filepath.Join(dir, "deployment.yaml"), `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.name }}
spec:
  replicas: {{ .Values.replicas }}
`)
	es := collect(t, CloudFormation{}, dir)
	if len(es) != 0 {
		t.Errorf("invalid YAML should produce no entries, got %+v", es)
	}
}

func TestCloudFormation_JSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "template.json"), `{
  "AWSTemplateFormatVersion": "2010-09-09",
  "Resources": {
    "Func": {
      "Type": "AWS::Lambda::Function",
      "Properties": {
        "Runtime": "python3.13"
      }
    }
  }
}`)
	es := collect(t, CloudFormation{}, dir)
	if len(es) != 1 || es[0].Type != "Lambda" || es[0].Version != "python3.13" {
		t.Errorf("got %+v", es)
	}
}
