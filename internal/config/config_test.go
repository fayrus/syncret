package config

import (
	"strings"
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
		envProvider:           "aws",
		envAWSSecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret/prod/db",
		envAWSRegion:          "us-east-1",
		envAWSTargetSecretARN: "arn:aws:secretsmanager:us-east-1:123:secret/prod/target",
		envTargetSecretKeys:   "password",
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
	if cfg.Timezone != "UTC" {
		t.Errorf("Timezone = %q, want UTC", cfg.Timezone)
	}
	if cfg.InstanceName != "" {
		t.Errorf("InstanceName = %q, want empty", cfg.InstanceName)
	}
}

func TestLoad_missingRequired(t *testing.T) {
	tests := []struct {
		name    string
		drop    string
		wantErr string
	}{
		{"missing secret ARN", envAWSSecretARN, envAWSSecretARN},
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

func TestLoad_provider(t *testing.T) {
	t.Run("missing provider returns error", func(t *testing.T) {
		env := baseEnv()
		delete(env, envProvider)
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error for missing provider, got nil")
		}
	})

	t.Run("unsupported provider returns error", func(t *testing.T) {
		env := baseEnv()
		env[envProvider] = "alibaba"
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error for unsupported provider, got nil")
		}
	})

	t.Run("aws provider is valid", func(t *testing.T) {
		setEnv(t, baseEnv())

		_, err := Load()
		if err != nil {
			t.Fatalf("Load() unexpected error for aws provider: %v", err)
		}
	})
}

func TestLoad_atLeastOneAction(t *testing.T) {
	t.Run("neither target secret nor ECS returns error", func(t *testing.T) {
		env := baseEnv()
		delete(env, envAWSTargetSecretARN)
		delete(env, envTargetSecretKeys)
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error when no action configured, got nil")
		}
	})

	t.Run("ECS only without target secret is valid", func(t *testing.T) {
		env := baseEnv()
		delete(env, envAWSTargetSecretARN)
		delete(env, envTargetSecretKeys)
		env[envAWSECSForceDeploy] = "true"
		env[envAWSECSCluster] = "my-cluster"
		env[envAWSECSServices] = "app"
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
			env[envAWSECSForceDeploy] = "true"
			env[envAWSECSCluster] = "my-cluster"
			env[envAWSECSServices] = tt.services
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

func TestLoad_notificationContext(t *testing.T) {
	tests := []struct {
		name         string
		instanceName string
		timezone     string
		wantErr      bool
	}{
		{
			name:         "instance name and Lima timezone are accepted",
			instanceName: "V4 Production",
			timezone:     "America/Lima",
		},
		{
			name:         "New York timezone is accepted",
			instanceName: "US Production",
			timezone:     "America/New_York",
		},
		{
			name:     "invalid timezone is rejected",
			timezone: "Mars/Olympus_Mons",
			wantErr:  true,
		},
		{
			name:         "multiline instance name is rejected",
			instanceName: "Production\nInjected",
			timezone:     "UTC",
			wantErr:      true,
		},
		{
			name:         "carriage return alone in instance name is rejected",
			instanceName: "Production\rInjected",
			timezone:     "UTC",
			wantErr:      true,
		},
		{
			name:         "instance name longer than 64 characters is rejected",
			instanceName: strings.Repeat("a", 65),
			timezone:     "UTC",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := baseEnv()
			env[envInstanceName] = tt.instanceName
			env[envTimezone] = tt.timezone
			setEnv(t, env)

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if cfg.InstanceName != tt.instanceName {
				t.Errorf("InstanceName = %q, want %q", cfg.InstanceName, tt.instanceName)
			}
			if cfg.Timezone != tt.timezone {
				t.Errorf("Timezone = %q, want %q", cfg.Timezone, tt.timezone)
			}
		})
	}
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
		env[envAWSECSForceDeploy] = "yes"
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error for invalid ECS force deploy bool, got nil")
		}
	})

	t.Run("enabled requires ECS cluster", func(t *testing.T) {
		env := baseEnv()
		env[envAWSECSForceDeploy] = "true"
		env[envAWSECSServices] = "syncret-db"
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error when ECS cluster missing, got nil")
		}
	})

	t.Run("enabled requires ECS services", func(t *testing.T) {
		env := baseEnv()
		env[envAWSECSForceDeploy] = "true"
		env[envAWSECSCluster] = "my-cluster"
		setEnv(t, env)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() expected error when ECS services missing, got nil")
		}
	})

	t.Run("enabled with single service", func(t *testing.T) {
		env := baseEnv()
		env[envAWSECSForceDeploy] = "true"
		env[envAWSECSCluster] = "my-cluster"
		env[envAWSECSServices] = "syncret-db"
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
		env[envAWSECSForceDeploy] = "true"
		env[envAWSECSCluster] = "my-cluster"
		env[envAWSECSServices] = "svc1,svc2,svc3"
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
