package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fayrus/syncret/internal/logctx"
)

type ECSClient interface {
	UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
}

type ECS struct {
	client ECSClient
}

func NewECS(client ECSClient) *ECS {
	return &ECS{client: client}
}

const maxDescribeBatch = 10

func (e *ECS) describeAll(ctx context.Context, cluster string, services []string) ([]types.Service, error) {
	var all []types.Service
	for i := 0; i < len(services); i += maxDescribeBatch {
		end := i + maxDescribeBatch
		if end > len(services) {
			end = len(services)
		}
		out, err := e.client.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(cluster),
			Services: services[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("ecs: describe services: %w", err)
		}
		all = append(all, out.Services...)
	}
	return all, nil
}

// Services not found return an error; inactive services are skipped with a warning.
func (e *ECS) ForceNewDeployment(ctx context.Context, cluster string, services []string) error {
	described, err := e.describeAll(ctx, cluster, services)
	if err != nil {
		return err
	}

	serviceByName := make(map[string]types.Service, len(described))
	for _, svc := range described {
		if svc.ServiceName != nil {
			serviceByName[*svc.ServiceName] = svc
		}
	}

	var errs []error
	for _, svc := range services {
		sd, found := serviceByName[svc]
		if !found {
			errs = append(errs, fmt.Errorf("ecs: service %q not found in cluster %q", svc, cluster))
			continue
		}
		if sd.Status == nil || *sd.Status != "ACTIVE" {
			logctx.From(ctx).Warn("ecs: service not active, skipping", "cluster", cluster, "service", svc)
			continue
		}

		logctx.From(ctx).Info("ecs: forcing new deployment", "cluster", cluster, "service", svc)

		_, err := e.client.UpdateService(ctx, &ecs.UpdateServiceInput{
			Cluster:            aws.String(cluster),
			Service:            aws.String(svc),
			ForceNewDeployment: true,
		})
		if err != nil {
			logctx.From(ctx).Error("ecs: force deploy failed", "cluster", cluster, "service", svc, "error", err)
			errs = append(errs, fmt.Errorf("ecs: force deploy %s/%s: %w", cluster, svc, err))
			continue
		}

		logctx.From(ctx).Info("ecs: deployment initiated", "cluster", cluster, "service", svc)
	}

	return errors.Join(errs...)
}

