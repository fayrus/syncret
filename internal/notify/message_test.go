package notify

import (
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 6, 20, 18, 6, 17, 0, time.UTC)

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		payload  Payload
		contains []string
		absent   []string
	}{
		{
			name: "success includes instance resources and localized date",
			payload: Payload{
				InstanceName: "V4 Production",
				EventName:    "RotationSucceeded",
				AccountID:    "123456789012",
				Region:       "us-east-1",
				SourceSecret: "example-source",
				TargetSecret: "example-target",
				ECSCluster:   "my-cluster",
				ECSServices:  []string{"backend", "worker"},
				RequestID:    "request-123",
				Actions: []string{
					"✅ Target secret updated",
					"✅ ECS force-deployment request accepted",
				},
				Status:       StatusSuccess,
				OccurredAt:   testTime,
				TimezoneName: "America/Lima",
			},
			contains: []string{
				"Syncret — V4 Production",
				"RotationSucceeded",
				"123456789012",
				"us-east-1",
				"example-source",
				"example-target",
				"my-cluster",
				"backend, worker",
				"2026-06-20 13:06:17 -05:00 (America/Lima)",
				"request-123",
				"*Actions:*\n✅ Target secret updated\n✅ ECS force-deployment request accepted\n",
				"*Status:* ✅ Success",
			},
			absent: []string{"arn:aws:", ":*  ", "•"},
		},
		{
			name: "failure includes sanitized error and event secret mismatch",
			payload: Payload{
				EventName:    "PutSecretValue",
				SourceSecret: "configured-source",
				EventSecret:  "unexpected-source",
				RequestID:    "request-456",
				Actions:      []string{"❌ Event rejected by source secret guard"},
				Status:       StatusFailed,
				ErrorMessage: "Event secret does not match the configured source secret; see Lambda logs using the request ID",
				OccurredAt:   testTime,
				TimezoneName: "UTC",
			},
			contains: []string{
				"configured-source",
				"unexpected-source",
				"request-456",
				"*Status:* ❌ Failed",
				"*Error:* Event secret does not match",
			},
			absent: []string{"✅ Success", ":*  "},
		},
		{
			name: "skipped status omits optional resources",
			payload: Payload{
				EventName:    "RotationFailed",
				SourceSecret: "example-source",
				Status:       StatusSkipped,
				OccurredAt:   testTime,
				TimezoneName: "UTC",
			},
			contains: []string{"RotationFailed", "⚠️ Skipped"},
			absent:   []string{"*Actions:*", "*Target secret:*", "*ECS cluster:*", "*ECS services:*"},
		},
		{
			name: "New York timezone applies daylight saving offset",
			payload: Payload{
				EventName:    "PutSecretValue",
				SourceSecret: "example-source",
				Status:       StatusSuccess,
				OccurredAt:   testTime,
				TimezoneName: "America/New_York",
			},
			contains: []string{"2026-06-20 14:06:17 -04:00 (America/New_York)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := format(tt.payload)
			for _, want := range tt.contains {
				if !strings.Contains(msg, want) {
					t.Errorf("format() missing %q\ngot:\n%s", want, msg)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(msg, absent) {
					t.Errorf("format() should not contain %q\ngot:\n%s", absent, msg)
				}
			}
		})
	}
}
