package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gitlab.scitix-inner.ai/k8s/aegis/api/models"
	"k8s.io/klog/v2"
)

var handlerMap map[string]interface{} = make(map[string]interface{})

func RegisterHandler(path string, handler func(http.ResponseWriter, *http.Request, func(ctx context.Context, alert *models.Alert) error)) {
	if _, ok := handlerMap[path]; !ok {
		handlerMap[path] = handler
		klog.Infof("Succeeded register route %s", path)
	} else {
		klog.Errorf("Failed to register existed route %s", path)
	}
}

func RunHttpServer(port, routePrefix string,
	createAlertHandler func(ctx context.Context, alert *models.Alert) error) {

	klog.Infof("Starting http server on port %s", port)
	mux := http.NewServeMux()

	routePrefix = strings.TrimSuffix(routePrefix, "/")

	for path, handler := range handlerMap {
		func(path string, handler interface{}) {
			mux.HandleFunc(routePrefix+path, func(rw http.ResponseWriter, r *http.Request) {
				h := handler.(func(http.ResponseWriter, *http.Request, func(ctx context.Context, alert *models.Alert) error))
				h(rw, r, createAlertHandler)
			})
		}(path, handler)
	}

	// Setup the prometheus metrics machinery
	prometheus.MustRegister(collectors.NewBuildInfoCollector())
	mux.Handle("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	))

	// start http server
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		klog.Fatalf("Starting http server failed: %v", err)
	}
}
