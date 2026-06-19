package event

import (
	"testing"
)

const testSecretARN = "arn:aws:secretsmanager:us-east-1:123456789:secret/prod/db-password"

func TestParse(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		wantEventName EventName
		wantSecretARN string
		wantErr       bool
	}{
		{
			name: "rotation succeeded — ARN in additionalEventData",
			payload: `{
				"source": "aws.secretsmanager",
				"detail-type": "AWS Service Event via CloudTrail",
				"detail": {
					"eventSource": "secretsmanager.amazonaws.com",
					"eventName": "RotationSucceeded",
					"requestParameters": null,
					"additionalEventData": {"SecretId": "` + testSecretARN + `"}
				}
			}`,
			wantEventName: RotationSucceeded,
			wantSecretARN: testSecretARN,
		},
		{
			name: "put secret value — ARN in requestParameters",
			payload: `{
				"source": "aws.secretsmanager",
				"detail-type": "AWS API Call via CloudTrail",
				"detail": {
					"eventSource": "secretsmanager.amazonaws.com",
					"eventName": "PutSecretValue",
					"requestParameters": {"secretId": "` + testSecretARN + `"}
				}
			}`,
			wantEventName: PutSecretValue,
			wantSecretARN: testSecretARN,
		},
		{
			name: "rotation failed — ARN in additionalEventData",
			payload: `{
				"source": "aws.secretsmanager",
				"detail": {
					"eventSource": "secretsmanager.amazonaws.com",
					"eventName": "RotationFailed",
					"requestParameters": null,
					"additionalEventData": {"SecretId": "` + testSecretARN + `"}
				}
			}`,
			wantEventName: RotationFailed,
			wantSecretARN: testSecretARN,
		},
		{
			name: "update secret version stage is rejected",
			payload: `{
				"source": "aws.secretsmanager",
				"detail": {
					"eventSource": "secretsmanager.amazonaws.com",
					"eventName": "UpdateSecretVersionStage",
					"requestParameters": {"secretId": "` + testSecretARN + `"}
				}
			}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			payload: `{invalid}`,
			wantErr: true,
		},
		{
			name:    "invalid top-level source",
			payload: `{"source": "attacker", "detail": {"eventSource": "secretsmanager.amazonaws.com", "eventName": "RotationSucceeded", "requestParameters": {"secretId": "arn:x"}}}`,
			wantErr: true,
		},
		{
			name:    "invalid event source rejected",
			payload: `{"source": "aws.secretsmanager", "detail": {"eventSource": "ec2.amazonaws.com", "eventName": "RotationSucceeded", "additionalEventData": {"SecretId": "arn:x"}}}`,
			wantErr: true,
		},
		{
			name:    "unknown event name",
			payload: `{"source": "aws.secretsmanager", "detail": {"eventSource": "secretsmanager.amazonaws.com", "eventName": "DeleteSecret", "requestParameters": {"secretId": "arn:x"}}}`,
			wantErr: true,
		},
		{
			name:    "missing secret id",
			payload: `{"source": "aws.secretsmanager", "detail": {"eventSource": "secretsmanager.amazonaws.com", "eventName": "RotationSucceeded", "requestParameters": null, "additionalEventData": {}}}`,
			wantErr: true,
		},
		{
			name:    "empty payload",
			payload: `{}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.payload))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Name != tt.wantEventName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantEventName)
			}
			if got.SecretARN != tt.wantSecretARN {
				t.Errorf("SecretARN = %q, want %q", got.SecretARN, tt.wantSecretARN)
			}
		})
	}
}

func TestShouldRedeploy(t *testing.T) {
	tests := []struct {
		name EventName
		want bool
	}{
		{RotationSucceeded, true},
		{PutSecretValue, true},
		{RotationFailed, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			e := &Event{Name: tt.name, SecretARN: testSecretARN}
			if got := e.ShouldRedeploy(); got != tt.want {
				t.Errorf("ShouldRedeploy() = %v, want %v", got, tt.want)
			}
		})
	}
}
