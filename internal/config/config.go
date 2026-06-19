package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	envSecretARN      = "SYNCRET_SECRET_ARN"
	envECSCluster     = "SYNCRET_ECS_CLUSTER"
	envECSServices    = "SYNCRET_ECS_SERVICES"
	envECSForceDeploy = "SYNCRET_ECS_FORCE_DEPLOY"
	envAWSRegion      = "SYNCRET_AWS_REGION"

	envTargetSecretARN  = "SYNCRET_TARGET_SECRET_ARN"
	envTargetSecretKeys = "SYNCRET_TARGET_SECRET_KEYS"

	envLogLevel  = "SYNCRET_LOG_LEVEL"
	envLogFormat = "SYNCRET_LOG_FORMAT"
)

type Config struct {
	SecretARN      string
	ECSCluster     string
	ECSServices    []string
	ECSForceDeploy bool
	AWSRegion      string

	TargetSecretARN  string
	TargetSecretKeys []string

	LogLevel  string
	LogFormat string
}

func Load() (*Config, error) {
	cfg := &Config{
		LogLevel:  envOrDefault(envLogLevel, "info"),
		LogFormat: envOrDefault(envLogFormat, "json"),

		SecretARN:  os.Getenv(envSecretARN),
		ECSCluster: os.Getenv(envECSCluster),
		AWSRegion:  os.Getenv(envAWSRegion),

		TargetSecretARN: os.Getenv(envTargetSecretARN),
	}

	var err error
	cfg.ECSForceDeploy, err = parseBoolOrDefault(envECSForceDeploy, false)
	if err != nil {
		return nil, err
	}

	cfg.ECSServices = parseList(os.Getenv(envECSServices))
	cfg.TargetSecretKeys = parseList(os.Getenv(envTargetSecretKeys))

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	for _, r := range []struct{ name, value string }{
		{envSecretARN, c.SecretARN},
		{envAWSRegion, c.AWSRegion},
	} {
		if r.value == "" {
			return fmt.Errorf("config: %s is required", r.name)
		}
	}

	if c.TargetSecretARN != "" && len(c.TargetSecretKeys) == 0 {
		return fmt.Errorf("config: %s is required when %s is set", envTargetSecretKeys, envTargetSecretARN)
	}

	if c.TargetSecretARN == "" && !c.ECSForceDeploy {
		return fmt.Errorf("config: at least one of %s or %s=true must be configured", envTargetSecretARN, envECSForceDeploy)
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
			return fmt.Errorf("config: %s is required when %s is true", envECSCluster, envECSForceDeploy)
		}
		if len(c.ECSServices) == 0 {
			return fmt.Errorf("config: %s is required when %s is true", envECSServices, envECSForceDeploy)
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
