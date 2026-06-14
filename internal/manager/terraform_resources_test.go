package manager

import (
	"path/filepath"
	"sort"
	"testing"
)

// A directory that can't be read is a genuine I/O failure and must surface as
// an error rather than be silently mistaken for an empty module.
func TestTerraformResources_DirReadError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := (TerraformResources{}).Extract(File{Path: missing}); err == nil {
		t.Fatal("want error for unreadable module directory, got nil")
	}
}

// RDS / Aurora / ElastiCache pick a default engine version when engine_version
// is omitted, which is common in real configs. A missing engine_version must
// not leak the engine name into the VERSION column.
func TestTerraformResources_MissingEngineVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.tf"), `
resource "aws_db_instance" "api" {
  engine         = "mysql"
  instance_class = "db.t3.micro"
}
`)
	es := collect(t, TerraformResources{}, dir)
	if len(es) != 0 {
		t.Errorf("want no entries when engine_version is missing, got %+v", es)
	}
}

func TestTerraformResources(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.tf"), `
resource "aws_lambda_function" "api" {
  function_name = "api"
  runtime       = "nodejs22.x"
}

resource "aws_lambda_layer_version" "shared" {
  layer_name          = "shared"
  compatible_runtimes = ["nodejs22.x", "nodejs20.x"]
}

resource "aws_db_instance" "primary" {
  engine         = "mysql"
  engine_version = "8.0.35"
}

resource "aws_rds_cluster" "aurora" {
  engine         = "aurora-mysql"
  engine_version = "8.0.mysql_aurora.3.06.0"
}

resource "aws_rds_cluster" "mysql_cluster" {
  engine         = "mysql"
  engine_version = "8.0.mysql_aurora.3.06.0"
}

resource "aws_elasticache_replication_group" "cache" {
  engine         = "redis"
  engine_version = "7.1"
}

resource "aws_elasticache_cluster" "memcache" {
  engine         = "memcached"
  engine_version = "1.6"
}

resource "aws_opensearch_domain" "search" {
  engine_version = "OpenSearch_2.11"
}

resource "aws_eks_cluster" "k8s" {
  version = "1.29"
}

resource "aws_eks_node_group" "workers" {
  release_version = "1.29.0-20240202"
}

resource "aws_lambda_function" "uses_var" {
  function_name = "uses_var"
  runtime       = var.lambda_runtime   # not a literal -> skipped
}

resource "google_cloud_run_v2_service" "api" {
  template {
    containers {
      image = "us-docker.pkg.dev/proj/repo/api:v1.2.3"
    }
    containers {
      image = "us-docker.pkg.dev/proj/repo/sidecar:0.5"
    }
  }
}

resource "google_cloud_run_v2_job" "etl" {
  template {
    template {
      containers {
        image = "us-docker.pkg.dev/proj/repo/etl:2024-01"
      }
    }
  }
}

resource "aws_ecs_task_definition" "app" {
  family = "app"
  container_definitions = jsonencode([
    {
      name  = "app"
      image = "nginx:1.25"
    },
    {
      name  = "datadog"
      image = "datadog/agent:7.50.0"
    }
  ])
}

resource "aws_apprunner_service" "web" {
  source_configuration {
    image_repository {
      image_identifier = "public.ecr.aws/myorg/web:1.4.2"
    }
  }
}

resource "google_cloudfunctions_function" "v1" {
  runtime = "nodejs20"
}

resource "google_cloudfunctions2_function" "v2" {
  build_config {
    runtime = "python311"
  }
}

resource "google_app_engine_standard_app_version" "std" {
  runtime = "ruby32"
}

resource "google_sql_database_instance" "db" {
  database_version = "POSTGRES_15"
}

resource "aws_instance" "bastion" {
  ami           = "ami-0123456789abcdef0"
  instance_type = "t3.micro"
}

resource "aws_launch_template" "asg" {
  image_id = "ami-fedcba9876543210f"
}

resource "google_compute_instance" "vm" {
  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
    }
  }
}

resource "aws_lambda_function" "edge_handler" {
  function_name = "edge_handler"
  runtime       = "nodejs22.x"
}

resource "aws_cloudfront_distribution" "site" {
  default_cache_behavior {
    lambda_function_association {
      event_type = "origin-request"
      lambda_arn = aws_lambda_function.edge_handler.qualified_arn
    }
  }
}

# lambda_arn here is a RelativeTraversalExpr whose Source has no
# Variables() — collectEdgeFunctions must not panic on it.
resource "aws_cloudfront_distribution" "weird" {
  default_cache_behavior {
    lambda_function_association {
      event_type = "viewer-request"
      lambda_arn = tolist(["arn:aws:lambda:us-east-1:111:function:x"])[0]
    }
  }
}
`)
	files := matched(t, TerraformResources{}, dir)
	if len(files) != 1 {
		t.Fatalf("detect: %v", files)
	}
	es := collect(t, TerraformResources{}, dir)

	type pair struct{ Type, Version string }
	got := map[pair]bool{}
	for _, e := range es {
		got[pair{e.Type, e.Version}] = true
	}
	want := []pair{
		{"Lambda", "nodejs22.x"},
		{"Lambda", "nodejs20.x"},
		{"RDS", "mysql 8.0.35"},
		{"Aurora", "aurora-mysql 8.0.mysql_aurora.3.06.0"},
		{"RDS", "mysql 8.0.mysql_aurora.3.06.0"}, // mysql cluster goes back to RDS
		{"ElastiCache (Redis)", "7.1"},
		{"ElastiCache (Memcached)", "1.6"},
		{"OpenSearch", "OpenSearch_2.11"},
		{"EKS", "1.29"},
		{"EKS Node Group", "1.29.0-20240202"},
		{"Cloud Run", "us-docker.pkg.dev/proj/repo/api:v1.2.3"},
		{"Cloud Run", "us-docker.pkg.dev/proj/repo/sidecar:0.5"},
		{"Cloud Run", "us-docker.pkg.dev/proj/repo/etl:2024-01"},
		{"ECS", "nginx:1.25"},
		{"ECS", "datadog/agent:7.50.0"},
		{"App Runner", "public.ecr.aws/myorg/web:1.4.2"},
		{"Cloud Functions", "nodejs20"},
		{"Cloud Functions", "python311"},
		{"App Engine", "ruby32"},
		{"Cloud SQL", "POSTGRES_15"},
		{"AMI", "ami-0123456789abcdef0"},
		{"AMI", "ami-fedcba9876543210f"},
		{"Compute Image", "debian-cloud/debian-12"},
		{"Lambda@Edge", "nodejs22.x"},
	}
	if len(got) != len(want) {
		var keys []string
		for p := range got {
			keys = append(keys, p.Type+"="+p.Version)
		}
		sort.Strings(keys)
		t.Errorf("got %d entries (%v), want %d", len(got), keys, len(want))
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing %+v", w)
		}
	}
}
