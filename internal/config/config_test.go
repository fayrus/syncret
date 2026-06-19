package config

import (
	"testing"
)

func setEnv(t *testing.T, kvs map[string]string) {
	t.Helper()
	for k, v := range kvs {
		t.Setenv(k, v)
	}
}

func baseEnv() map[string]string {
	return map[string]string{
		envSecretARN:        "arn:aws:secretsmanager:us-east-1:123:secret/prod/db",
		envAWSRegion:        "us-east-1",
		envTargetSecretARN:  "arn:aws:secretsmanager:us-east-1:123:secret/prod/target",
		envTargetSecretKeys: "password",
	}
}

func TestLoad_defaults(t *testing.T) {
	setEnv(t, baseEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want json", cfg.LogFormat)
	}
}

func TestLoad_missingRequired(t *testing.T) {
	tests := []struct {
		name    string
		drop    string
		wantErr string
	}{
		{"missing secret ARN", envSecretARN, envSecretARN},
		{"missing AWS region", envAWSRegion, envAWSRegion},
		{"missing target secret keys when ARN set", envTargetSecretKeys, envTargetSecretKeys},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := baseEnv()
			delete(env, tt.drop)
			setEnv(t, env)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() expected error, got nil")
			}
		})
	}
}

func TestLoad_atLeastOneAction(t *testing.T) {
	t.Run("neither target secret nor ECS returns error", func(t *testing.T) {
		env := baseEnv()
		delete(env, envTargetSecretARN)
		delete(env, envTargetSecretKeys)
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error when no action configured, got nil")
		}
	})

	t.Run("ECS only without target secret is valid", func(t *testing.T) {
		env := baseEnv()
		delete(env, envTargetSecretARN)
		delete(env, envTargetSecretKeys)
		env[envECSForceDeploy] = "true"
		env[envECSCluster] = "my-cluster"
		env[envECSServices] = "app"
		setEnv(t, env)

		_, err := Load()
		if err != nil {
			t.Fatalf("Load() unexpected error for ECS-only config: %v", err)
		}
	})

	t.Run("target secret ARN without keys returns error", func(t *testing.T) {
		env := baseEnv()
		delete(env, envTargetSecretKeys)
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error when target ARN set but keys missing, got nil")
		}
	})
}

func TestLoad_targetSecret(t *testing.T) {
	t.Run("multiple keys parsed correctly", func(t *testing.T) {
		env := baseEnv()
		env[envTargetSecretKeys] = "username,password"
		setEnv(t, env)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}
		if len(cfg.TargetSecretKeys) != 2 {
			t.Errorf("TargetSecretKeys len = %d, want 2", len(cfg.TargetSecretKeys))
		}
	})
}

func TestLoad_parseList(t *testing.T) {
	tests := []struct {
		name     string
		services string
		keys     string
		wantSvcs []string
		wantKeys []string
	}{
		{
			name:     "spaces around elements are trimmed",
			services: "api, worker",
			keys:     "password, username",
			wantSvcs: []string{"api", "worker"},
			wantKeys: []string{"password", "username"},
		},
		{
			name:     "trailing comma is ignored",
			services: "api,",
			keys:     "password,",
			wantSvcs: []string{"api"},
			wantKeys: []string{"password"},
		},
		{
			name:     "empty elements are dropped",
			services: "api,,worker",
			keys:     "password,,backup",
			wantSvcs: []string{"api", "worker"},
			wantKeys: []string{"password", "backup"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := baseEnv()
			env[envECSForceDeploy] = "true"
			env[envECSCluster] = "my-cluster"
			env[envECSServices] = tt.services
			env[envTargetSecretKeys] = tt.keys
			setEnv(t, env)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if len(cfg.ECSServices) != len(tt.wantSvcs) {
				t.Errorf("ECSServices = %v, want %v", cfg.ECSServices, tt.wantSvcs)
			}
			for i, s := range cfg.ECSServices {
				if s != tt.wantSvcs[i] {
					t.Errorf("ECSServices[%d] = %q, want %q", i, s, tt.wantSvcs[i])
				}
			}
			if len(cfg.TargetSecretKeys) != len(tt.wantKeys) {
				t.Errorf("TargetSecretKeys = %v, want %v", cfg.TargetSecretKeys, tt.wantKeys)
			}
			for i, k := range cfg.TargetSecretKeys {
				if k != tt.wantKeys[i] {
					t.Errorf("TargetSecretKeys[%d] = %q, want %q", i, k, tt.wantKeys[i])
				}
			}
		})
	}
}

func TestLoad_logConfig(t *testing.T) {
	t.Run("invalid log level returns error", func(t *testing.T) {
		env := baseEnv()
		env[envLogLevel] = "verbose"
		setEnv(t, env)
		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error for invalid log level, got nil")
		}
	})

	t.Run("invalid log format returns error", func(t *testing.T) {
		env := baseEnv()
		env[envLogFormat] = "yaml"
		setEnv(t, env)
		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error for invalid log format, got nil")
		}
	})

	t.Run("valid log levels are accepted", func(t *testing.T) {
		for _, level := range []string{"debug", "info", "warn", "error"} {
			env := baseEnv()
			env[envLogLevel] = level
			setEnv(t, env)
			if _, err := Load(); err != nil {
				t.Errorf("Load() unexpected error for level %q: %v", level, err)
			}
		}
	})

	t.Run("valid log formats are accepted", func(t *testing.T) {
		for _, format := range []string{"json", "text"} {
			env := baseEnv()
			env[envLogFormat] = format
			setEnv(t, env)
			if _, err := Load(); err != nil {
				t.Errorf("Load() unexpected error for format %q: %v", format, err)
			}
		}
	})
}

func TestLoad_ecsForceDeploy(t *testing.T) {
	t.Run("disabled by default does not require ECS config", func(t *testing.T) {
		setEnv(t, baseEnv())

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}
		if cfg.ECSForceDeploy {
			t.Error("ECSForceDeploy = true, want false")
		}
	})

	t.Run("invalid bool returns error", func(t *testing.T) {
		env := baseEnv()
		env[envECSForceDeploy] = "yes"
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error for invalid ECS force deploy bool, got nil")
		}
	})

	t.Run("enabled requires ECS cluster", func(t *testing.T) {
		env := baseEnv()
		env[envECSForceDeploy] = "true"
		env[envECSServices] = "syncret-db"
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error when ECS cluster missing, got nil")
		}
	})

	t.Run("enabled requires ECS services", func(t *testing.T) {
		env := baseEnv()
		env[envECSForceDeploy] = "true"
		env[envECSCluster] = "my-cluster"
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error when ECS services missing, got nil")
		}
	})

	t.Run("enabled with single service", func(t *testing.T) {
		env := baseEnv()
		env[envECSForceDeploy] = "true"
		env[envECSCluster] = "my-cluster"
		env[envECSServices] = "syncret-db"
		setEnv(t, env)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}
		if len(cfg.ECSServices) != 1 || cfg.ECSServices[0] != "syncret-db" {
			t.Errorf("ECSServices = %v, want [syncret-db]", cfg.ECSServices)
		}
	})

	t.Run("enabled with multiple services", func(t *testing.T) {
		env := baseEnv()
		env[envECSForceDeploy] = "true"
		env[envECSCluster] = "my-cluster"
		env[envECSServices] = "svc1,svc2,svc3"
		setEnv(t, env)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}
		if len(cfg.ECSServices) != 3 {
			t.Errorf("ECSServices len = %d, want 3", len(cfg.ECSServices))
		}
	})
}
