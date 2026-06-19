package aws

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type mockSecretsClient struct {
	getOut *secretsmanager.GetSecretValueOutput
	getErr error
	putErr error

	putCalledWith *secretsmanager.PutSecretValueInput
}

func (m *mockSecretsClient) GetSecretValue(_ context.Context, params *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	return m.getOut, m.getErr
}

func (m *mockSecretsClient) PutSecretValue(_ context.Context, params *secretsmanager.PutSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	m.putCalledWith = params
	return &secretsmanager.PutSecretValueOutput{}, m.putErr
}

func secretString(v map[string]any) *secretsmanager.GetSecretValueOutput {
	raw, _ := json.Marshal(v)
	return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(string(raw))}
}

// ── GetSecretString ──────────────────────────────────────────────────────────

func TestGetSecretString(t *testing.T) {
	tests := []struct {
		name    string
		out     *secretsmanager.GetSecretValueOutput
		err     error
		want    string
		wantErr bool
	}{
		{
			name: "returns secret string",
			out:  &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"password":"abc"}`)},
			want: `{"password":"abc"}`,
		},
		{
			name:    "aws error propagated",
			err:     errors.New("access denied"),
			wantErr: true,
		},
		{
			name:    "nil secret string",
			out:     &secretsmanager.GetSecretValueOutput{SecretString: nil},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSecretsManager(&mockSecretsClient{getOut: tt.out, getErr: tt.err})
			got, err := sm.GetSecretString(context.Background(), "arn:test")
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ── MergeAndPutSecret ────────────────────────────────────────────────────────

func TestMergeAndPutSecret(t *testing.T) {
	target := map[string]any{
		"host":     "db.example.com",
		"port":     "5432",
		"dbname":   "mydb",
		"username": "app",
		"password": "old",
	}
	source := map[string]any{
		"username": "app",
		"password": "newpass",
	}

	tests := []struct {
		name      string
		target    map[string]any
		source    map[string]any
		keys      []string
		getErr    error
		putErr    error
		wantErr   bool
		wantNoPut bool
		checkPut  func(t *testing.T, input *secretsmanager.PutSecretValueInput)
	}{
		{
			name:    "updates single key",
			target:  target,
			source:  source,
			keys:    []string{"password"},
			wantErr: false,
			checkPut: func(t *testing.T, input *secretsmanager.PutSecretValueInput) {
				var m map[string]any
				if err := json.Unmarshal([]byte(*input.SecretString), &m); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if m["password"] != "newpass" {
					t.Errorf("password = %v, want newpass", m["password"])
				}
				if m["host"] != "db.example.com" {
					t.Errorf("host should be preserved, got %v", m["host"])
				}
			},
		},
		{
			name:    "updates multiple keys",
			target:  target,
			source:  source,
			keys:    []string{"username", "password"},
			wantErr: false,
			checkPut: func(t *testing.T, input *secretsmanager.PutSecretValueInput) {
				var m map[string]any
				if err := json.Unmarshal([]byte(*input.SecretString), &m); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if m["username"] != "app" {
					t.Errorf("username = %v, want app", m["username"])
				}
				if m["password"] != "newpass" {
					t.Errorf("password = %v, want newpass", m["password"])
				}
			},
		},
		{
			name:   "maps source key to different target key",
			target: map[string]any{"MB_DB_PASS": "old"},
			source: map[string]any{"password": "newpass"},
			keys:   []string{"password:MB_DB_PASS"},
			checkPut: func(t *testing.T, input *secretsmanager.PutSecretValueInput) {
				var m map[string]any
				if err := json.Unmarshal([]byte(*input.SecretString), &m); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if m["MB_DB_PASS"] != "newpass" {
					t.Errorf("MB_DB_PASS = %v, want newpass", m["MB_DB_PASS"])
				}
				if _, ok := m["password"]; ok {
					t.Error("source key 'password' should not appear in target")
				}
			},
		},
		{
			name:      "missing source key returns error without calling PutSecretValue",
			target:    target,
			source:    source,
			keys:      []string{"missing_key"},
			wantErr:   true,
			wantNoPut: true,
		},
		{
			name:    "get error propagated",
			target:  target,
			source:  source,
			keys:    []string{"password"},
			getErr:  errors.New("access denied"),
			wantErr: true,
		},
		{
			name:    "put error propagated",
			target:  target,
			source:  source,
			keys:    []string{"password"},
			putErr:  errors.New("throttled"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSecretsClient{
				getOut: secretString(tt.target),
				getErr: tt.getErr,
				putErr: tt.putErr,
			}
			sm := NewSecretsManager(mock)

			err := sm.MergeAndPutSecret(context.Background(), "arn:target", tt.source, tt.keys)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantNoPut && mock.putCalledWith != nil {
				t.Error("PutSecretValue must not be called")
			}
			if !tt.wantErr && tt.checkPut != nil {
				tt.checkPut(t, mock.putCalledWith)
			}
		})
	}
}

func TestMergeAndPutSecret_InvalidTargetJSON(t *testing.T) {
	mock := &mockSecretsClient{
		getOut: &secretsmanager.GetSecretValueOutput{
			SecretString: aws.String("not valid json"),
			VersionId:    aws.String("v1"),
		},
	}
	sm := NewSecretsManager(mock)
	err := sm.MergeAndPutSecret(context.Background(), "arn:target", map[string]any{"password": "new"}, []string{"password"})
	if err == nil {
		t.Fatal("expected error for non-JSON target secret, got nil")
	}
	if mock.putCalledWith != nil {
		t.Error("PutSecretValue must not be called when target JSON is invalid")
	}
}

// ── MergeAndPutSecret idempotency ────────────────────────────────────────────

func TestMergeAndPutSecret_ClientRequestToken(t *testing.T) {
	const versionID = "abc-123-version"
	raw, _ := json.Marshal(map[string]any{"password": "old"})
	getOut := &secretsmanager.GetSecretValueOutput{
		SecretString: aws.String(string(raw)),
		VersionId:    aws.String(versionID),
	}

	mock := &mockSecretsClient{getOut: getOut}
	sm := NewSecretsManager(mock)

	if err := sm.MergeAndPutSecret(context.Background(), "arn:target", map[string]any{"password": "new"}, []string{"password"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.putCalledWith == nil {
		t.Fatal("PutSecretValue was not called")
	}

	got := aws.ToString(mock.putCalledWith.ClientRequestToken)
	want := idempotencyToken("arn:target", versionID)
	if got != want {
		t.Errorf("ClientRequestToken = %q, want %q", got, want)
	}
}

func TestIdempotencyToken(t *testing.T) {
	a := idempotencyToken("arn:aws:secretsmanager:us-east-1:123:secret/prod", "v1")
	b := idempotencyToken("arn:aws:secretsmanager:us-east-1:123:secret/prod", "v1")
	c := idempotencyToken("arn:aws:secretsmanager:us-east-1:123:secret/prod", "v2")

	if a != b {
		t.Error("same inputs must produce same token")
	}
	if a == c {
		t.Error("different versionID must produce different token")
	}
	// UUID format: 8-4-4-4-12
	if len(a) != 36 {
		t.Errorf("token length = %d, want 36", len(a))
	}
}
