package aws

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type SecretsClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	PutSecretValue(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
}

type SecretsManager struct {
	client SecretsClient
}

func NewSecretsManager(client SecretsClient) *SecretsManager {
	return &SecretsManager{client: client}
}

func (s *SecretsManager) getSecretValue(ctx context.Context, arn string) (value, versionID string, err error) {
	out, err := s.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(arn),
	})
	if err != nil {
		return "", "", fmt.Errorf("secrets: get %s: %w", arn, err)
	}
	if out.SecretString == nil {
		return "", "", fmt.Errorf("secrets: %s has no string value", arn)
	}
	return *out.SecretString, aws.ToString(out.VersionId), nil
}

func (s *SecretsManager) GetSecretString(ctx context.Context, arn string) (string, error) {
	value, _, err := s.getSecretValue(ctx, arn)
	return value, err
}

// ClientRequestToken is derived from the target's current VersionId so that concurrent invocations
// processing the same event produce the same token — making the second write a no-op.
func (s *SecretsManager) MergeAndPutSecret(ctx context.Context, targetARN string, sourceData map[string]any, keys []string) error {
	raw, versionID, err := s.getSecretValue(ctx, targetARN)
	if err != nil {
		return err
	}

	var target map[string]any
	if err := json.Unmarshal([]byte(raw), &target); err != nil {
		return fmt.Errorf("secrets: parse target %s: %w", targetARN, err)
	}

	for _, k := range keys {
		srcKey, dstKey, _ := strings.Cut(k, ":")
		if dstKey == "" {
			dstKey = srcKey
		}
		v, ok := sourceData[srcKey]
		if !ok {
			return fmt.Errorf("secrets: source key %q not found in secret", srcKey)
		}
		target[dstKey] = v
	}

	updated, err := json.Marshal(target)
	if err != nil {
		return fmt.Errorf("secrets: marshal updated secret: %w", err)
	}

	_, err = s.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:           aws.String(targetARN),
		SecretString:       aws.String(string(updated)),
		ClientRequestToken: aws.String(idempotencyToken(targetARN, versionID)),
	})
	if err != nil {
		return fmt.Errorf("secrets: put %s: %w", targetARN, err)
	}

	return nil
}

// idempotencyToken returns a UUID-formatted string derived from targetARN and versionID.
// Secrets Manager requires ClientRequestToken to be in UUID format (≤64 chars, alphanumeric + hyphens).
func idempotencyToken(targetARN, versionID string) string {
	h := sha256.Sum256([]byte(targetARN + ":" + versionID))
	s := hex.EncodeToString(h[:16])
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
}
