package manager

import (
	"path/filepath"
	"testing"
)

// A directory that can't be read is a genuine I/O failure and must surface as
// an error rather than be silently mistaken for an empty module.
func TestTerraform_DirReadError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := (Terraform{}).Extract(File{Path: missing}); err == nil {
		t.Fatal("want error for unreadable module directory, got nil")
	}
}

func TestTerraform_Basic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.tf"), `
terraform {
  required_version = ">= 1.6.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    google = {
      source  = "hashicorp/google"
      version = ">= 5.10"
    }
  }
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.5.0"
}

module "local" {
  source = "./local-mod"
}
`)
	// terraform-config-inspect refuses to load a module without a module call
	// target if there's a missing local source, but it returns partial data.

	files := matched(t, Terraform{}, dir)
	if len(files) != 1 || files[0].Path != dir {
		t.Fatalf("detect: %+v", files)
	}
	es := collect(t, Terraform{}, dir)

	types := map[string]int{}
	got := map[string]string{}
	for _, e := range es {
		types[e.Type]++
		got[e.Type+":"+e.Name] = e.Version
	}
	if got["Terraform:required_version"] != ">= 1.6.0" {
		t.Errorf("required_version: %v", es)
	}
	if got["Terraform Provider:aws"] != "~> 5.0" {
		t.Errorf("aws provider: %v", es)
	}
	if got["Terraform Provider:google"] != ">= 5.10" {
		t.Errorf("google provider: %v", es)
	}
	if got["Terraform Module:vpc"] != "5.5.0" {
		t.Errorf("vpc module: %v", es)
	}
	if _, ok := got["Terraform Module:local"]; ok {
		t.Errorf("local module without version should be skipped")
	}
}
