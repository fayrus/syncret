package event

import (
	"encoding/json"
	"fmt"
)

type EventName string

const (
	RotationSucceeded EventName = "RotationSucceeded"
	PutSecretValue    EventName = "PutSecretValue"
	RotationFailed    EventName = "RotationFailed"
)

type Event struct {
	Name      EventName
	SecretARN string
}

func (e *Event) ShouldRedeploy() bool {
	return e.Name == RotationSucceeded || e.Name == PutSecretValue
}

type rawEvent struct {
	Source     string `json:"source"`
	DetailType string `json:"detail-type"`
	Detail     struct {
		EventSource       string `json:"eventSource"`
		EventName         string `json:"eventName"`
		RequestParameters struct {
			SecretID string `json:"secretId"`
		} `json:"requestParameters"`
		AdditionalEventData struct {
			SecretID string `json:"SecretId"`
		} `json:"additionalEventData"`
	} `json:"detail"`
}

func Parse(data []byte) (*Event, error) {
	var raw rawEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("event: parse: %w", err)
	}

	if raw.Source != "aws.secretsmanager" {
		return nil, fmt.Errorf("event: unexpected source %q", raw.Source)
	}

	if raw.Detail.EventSource != "secretsmanager.amazonaws.com" {
		return nil, fmt.Errorf("event: unexpected eventSource %q", raw.Detail.EventSource)
	}

	name := EventName(raw.Detail.EventName)
	switch name {
	case RotationSucceeded, PutSecretValue, RotationFailed:
	default:
		return nil, fmt.Errorf("event: unknown eventName %q", name)
	}

	var secretARN string
	switch name {
	case PutSecretValue:
		secretARN = raw.Detail.RequestParameters.SecretID
	default:
		secretARN = raw.Detail.AdditionalEventData.SecretID
	}

	if secretARN == "" {
		return nil, fmt.Errorf("event: missing secret ARN for %q", name)
	}

	return &Event{
		Name:      name,
		SecretARN: secretARN,
	}, nil
}
