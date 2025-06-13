package prom

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/scitix/aegis/tools"
	"k8s.io/klog/v2"
)

type PromAPI struct {
	API v1.API
}

var promAPI *PromAPI

func GetPromAPI() *PromAPI {
	if promAPI != nil {
		return promAPI
	}

	endpoint := tools.GetPrometheusEndpoint()
	client, err := api.NewClient(api.Config{
		Address: endpoint,
	})

	if err != nil {
		klog.Warningf("Failed to create client: %v", err)
	}

	promAPI = &PromAPI{
		API: v1.NewAPI(client),
	}

	return promAPI
}

func (a *PromAPI) Query(ctx context.Context, query string) (model.Value, error) {
	result, warnings, err := a.API.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	if len(warnings) > 0 {
		klog.V(4).Infof("Warnings: %v", warnings)
	}

	return result, nil
}

func (a *PromAPI) QueryRange(ctx context.Context, query string, offset string) (model.Value, error) {
	duration, err := time.ParseDuration(offset)
	if err != nil {
		return nil, fmt.Errorf("Invalid offset %s: %v", offset, err)
	}

	r := v1.Range{
		Start: time.Now().Add(-duration),
		End:   time.Now(),
		Step:  time.Minute,
	}

	result, warnings, err := a.API.QueryRange(ctx, query, r)
	if err != nil {
		return nil, err
	}

	if len(warnings) > 0 {
		klog.V(4).Infof("Warnings: %v", warnings)
	}

	return result, nil
}

func (a *PromAPI) QueryAndResovleTargetLabelValue(ctx context.Context, query string, label string) (string, error) {
	value, err := a.Query(ctx, query)
	if err != nil {
		return "", err
	}

	switch value.Type() {
	case model.ValVector:
		vectors := value.(model.Vector)
		if len(vectors) == 0 {
			return "", nil
		}
		metric := vectors[0].Metric
		return string(metric[model.LabelName(label)]), nil
	default:
		return "", fmt.Errorf("Unexpected value type: %v", value.Type())
	}
}

func (a *PromAPI) QueryAndResovleValue(ctx context.Context, query string) (float64, error) {
	value, err := a.Query(ctx, query)
	if err != nil {
		return 0, nil
	}

	switch value.Type() {
	case model.ValVector:
		vectors := value.(model.Vector)
		if len(vectors) == 0 {
			return 0, nil
		}
		return float64(vectors[0].Value), nil
	default:
		return 0, fmt.Errorf("Unexpected value type: %v", value.Type())
	}
}
