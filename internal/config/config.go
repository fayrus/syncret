package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	envProvider = "SYNCRET_PROVIDER"

	envAWSSecretARN      = "SYNCRET_AWS_SECRET_ARN"
	envAWSRegion         = "SYNCRET_AWS_REGION"
	envAWSTargetSecretARN = "SYNCRET_AWS_TARGET_SECRET_ARN"
	envAWSECSForceDeploy = "SYNCRET_AWS_ECS_FORCE_DEPLOY"
	envAWSECSCluster     = "SYNCRET_AWS_ECS_CLUSTER"
	envAWSECSServices    = "SYNCRET_AWS_ECS_SERVICES"

	envTargetSecretKeys = "SYNCRET_TARGET_SECRET_KEYS"
	envLogLevel         = "SYNCRET_LOG_LEVEL"
	envLogFormat        = "SYNCRET_LOG_FORMAT"

	envGChatWebhook = "SYNCRET_GCHAT_WEBHOOK"
)

type Config struct {
	Provider string

	SecretARN      string
	ECSCluster     string
	ECSServices    []string
	ECSForceDeploy bool
	AWSRegion      string

	TargetSecretARN  string
	TargetSecretKeys []string

	LogLevel  string
	LogFormat string

	GChatWebhook string
}

func Load() (*Config, error) {
	cfg := &Config{
		LogLevel:  envOrDefault(envLogLevel, "info"),
		LogFormat: envOrDefault(envLogFormat, "json"),

		Provider:  os.Getenv(envProvider),
		SecretARN: os.Getenv(envAWSSecretARN),
		AWSRegion: os.Getenv(envAWSRegion),
		ECSCluster: os.Getenv(envAWSECSCluster),

		TargetSecretARN: os.Getenv(envAWSTargetSecretARN),
		GChatWebhook:    os.Getenv(envGChatWebhook),
	}

	var err error
	cfg.ECSForceDeploy, err = parseBoolOrDefault(envAWSECSForceDeploy, false)
	if err != nil {
		return nil, err
	}

	cfg.ECSServices = parseList(os.Getenv(envAWSECSServices))
	cfg.TargetSecretKeys = parseList(os.Getenv(envTargetSecretKeys))

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	validProviders := map[string]bool{"aws": true}
	if !validProviders[c.Provider] {
		return fmt.Errorf("config: invalid %s %q: supported values: aws", envProvider, c.Provider)
	}

	for _, r := range []struct{ name, value string }{
		{envAWSSecretARN, c.SecretARN},
		{envAWSRegion, c.AWSRegion},
	} {
		if r.value == "" {
			return fmt.Errorf("config: %s is required", r.name)
		}
	}

	if c.TargetSecretARN != "" && len(c.TargetSecretKeys) == 0 {
		return fmt.Errorf("config: %s is required when %s is set", envTargetSecretKeys, envAWSTargetSecretARN)
	}

	if c.TargetSecretARN == "" && !c.ECSForceDeploy {
		return fmt.Errorf("config: at least one of %s or %s=true must be configured", envAWSTargetSecretARN, envAWSECSForceDeploy)
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("config: invalid %s %q: must be debug, info, warn, or error", envLogLevel, c.LogLevel)
	}
	validLogFormats := map[string]bool{"json": true, "text": true}
	if !validLogFormats[c.LogFormat] {
		return fmt.Errorf("config: invalid %s %q: must be json or text", envLogFormat, c.LogFormat)
	}

	if c.ECSForceDeploy {
		if c.ECSCluster == "" {
			return fmt.Errorf("config: %s is required when %s is true", envAWSECSCluster, envAWSECSForceDeploy)
		}
		if len(c.ECSServices) == 0 {
			return fmt.Errorf("config: %s is required when %s is true", envAWSECSServices, envAWSECSForceDeploy)
		}
	}

	return nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseList(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(s); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func parseBoolOrDefault(key string, def bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: invalid %s %q: must be boolean", key, v)
	}
	return b, nil
}
