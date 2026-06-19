package aws

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type mockECSClient struct {
	describeOut   *ecs.DescribeServicesOutput
	describeErr   error
	describeCalls int
	updateErr     error
	updateErrFor  string // if set, only this service name returns updateErr; others succeed
	updateCalls   []string
}

func (m *mockECSClient) DescribeServices(_ context.Context, params *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	m.describeCalls++
	return m.describeOut, m.describeErr
}

func (m *mockECSClient) UpdateService(_ context.Context, params *ecs.UpdateServiceInput, _ ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error) {
	m.updateCalls = append(m.updateCalls, *params.Service)
	if m.updateErrFor != "" {
		if *params.Service == m.updateErrFor {
			return &ecs.UpdateServiceOutput{}, m.updateErr
		}
		return &ecs.UpdateServiceOutput{}, nil
	}
	return &ecs.UpdateServiceOutput{}, m.updateErr
}

func activeService(name string) types.Service {
	return types.Service{ServiceName: aws.String(name), Status: aws.String("ACTIVE")}
}

func inactiveService(name string) types.Service {
	return types.Service{ServiceName: aws.String(name), Status: aws.String("DRAINING")}
}

func nilStatusService(name string) types.Service {
	return types.Service{ServiceName: aws.String(name), Status: nil}
}

func TestForceNewDeployment(t *testing.T) {
	tests := []struct {
		name              string
		services          []string
		describeOut       *ecs.DescribeServicesOutput
		describeErr       error
		updateErr         error
		updateErrFor      string
		wantErr           bool
		wantUpdated       []string
		wantDescribeCalls int
	}{
		{
			name:     "single active service is deployed",
			services: []string{"syncret-db"},
			describeOut: &ecs.DescribeServicesOutput{
				Services: []types.Service{activeService("syncret-db")},
			},
			wantUpdated: []string{"syncret-db"},
		},
		{
			name:     "multiple active services all deployed",
			services: []string{"api", "worker"},
			describeOut: &ecs.DescribeServicesOutput{
				Services: []types.Service{activeService("api"), activeService("worker")},
			},
			wantUpdated: []string{"api", "worker"},
		},
		{
			name:     "inactive service is skipped",
			services: []string{"api", "draining-svc"},
			describeOut: &ecs.DescribeServicesOutput{
				Services: []types.Service{activeService("api"), inactiveService("draining-svc")},
			},
			wantUpdated: []string{"api"},
		},
		{
			name:        "describe error is returned",
			services:    []string{"api"},
			describeErr: errors.New("permission denied"),
			wantErr:     true,
		},
		{
			name:     "update error is returned",
			services: []string{"api"},
			describeOut: &ecs.DescribeServicesOutput{
				Services: []types.Service{activeService("api")},
			},
			updateErr:   errors.New("throttled"),
			wantErr:     true,
			wantUpdated: []string{"api"},
		},
		{
			name:     "unknown service not in describe result returns error",
			services: []string{"ghost-svc"},
			describeOut: &ecs.DescribeServicesOutput{
				Services: []types.Service{},
			},
			wantErr:     true,
			wantUpdated: nil,
		},
		{
			name:     "multiple services fail — all errors returned",
			services: []string{"api", "worker"},
			describeOut: &ecs.DescribeServicesOutput{
				Services: []types.Service{activeService("api"), activeService("worker")},
			},
			updateErr:   errors.New("throttled"),
			wantErr:     true,
			wantUpdated: []string{"api", "worker"},
		},
		{
			name:     "nil status service is skipped",
			services: []string{"api", "nil-svc"},
			describeOut: &ecs.DescribeServicesOutput{
				Services: []types.Service{activeService("api"), nilStatusService("nil-svc")},
			},
			wantUpdated: []string{"api"},
		},
		{
			name:     "partial failure — successful services are still deployed",
			services: []string{"api", "worker"},
			describeOut: &ecs.DescribeServicesOutput{
				Services: []types.Service{activeService("api"), activeService("worker")},
			},
			updateErr:    errors.New("throttled"),
			updateErrFor: "worker",
			wantErr:      true,
			wantUpdated:  []string{"api", "worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockECSClient{
				describeOut:  tt.describeOut,
				describeErr:  tt.describeErr,
				updateErr:    tt.updateErr,
				updateErrFor: tt.updateErrFor,
			}
			e := NewECS(mock)

			err := e.ForceNewDeployment(context.Background(), "my-cluster", tt.services)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(mock.updateCalls) != len(tt.wantUpdated) {
				t.Fatalf("UpdateService called %d times, want %d", len(mock.updateCalls), len(tt.wantUpdated))
			}

			for i, svc := range tt.wantUpdated {
				if mock.updateCalls[i] != svc {
					t.Errorf("updateCalls[%d] = %q, want %q", i, mock.updateCalls[i], svc)
				}
			}

			if tt.wantDescribeCalls > 0 && mock.describeCalls != tt.wantDescribeCalls {
				t.Errorf("DescribeServices called %d times, want %d", mock.describeCalls, tt.wantDescribeCalls)
			}
		})
	}
}

func TestForceNewDeployment_Batching(t *testing.T) {
	const n = 11
	services := make([]string, n)
	described := make([]types.Service, n)
	for i := range services {
		services[i] = fmt.Sprintf("svc-%02d", i)
		described[i] = activeService(services[i])
	}

	mock := &mockECSClient{
		describeOut: &ecs.DescribeServicesOutput{Services: described},
	}
	e := NewECS(mock)

	if err := e.ForceNewDeployment(context.Background(), "my-cluster", services); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.describeCalls != 2 {
		t.Errorf("DescribeServices called %d times, want 2 (11 services batched in 10+1)", mock.describeCalls)
	}
	if len(mock.updateCalls) != n {
		t.Errorf("UpdateService called %d times, want %d", len(mock.updateCalls), n)
	}
}

