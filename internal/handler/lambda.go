package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/fayrus/syncret/internal/config"
	"github.com/fayrus/syncret/internal/logctx"
)

func StartLambda(cfg *config.Config, deps Dependencies) {
	lambda.Start(func(ctx context.Context, payload json.RawMessage) error {
		requestID := "unknown"
		if lc, ok := lambdacontext.FromContext(ctx); ok {
			requestID = lc.AwsRequestID
		}
		ctx = logctx.WithLogger(ctx, slog.Default().With("request_id", requestID))
		return handle(ctx, cfg, []byte(payload), deps)
	})
}
