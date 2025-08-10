package apis

import (
	"context"
	"net/http"

	"github.com/scitix/aegis/api"
	"github.com/scitix/aegis/api/models"
	"github.com/scitix/aegis/pkg/metrics"
	"k8s.io/klog/v2"
)

func init() {
	api.RegisterHandler("/alertmanager/alert", alertmanager)
}

func alertmanager(rw http.ResponseWriter, r *http.Request, callback func(ctx context.Context, alert *models.Alert) error,
	metrics *metrics.MetricsController,
) {
	source := string(models.AlertManagerAlertSource)

	response := api.CommonResponse{
		Code: api.OK,
	}
	defer func() {
		api.EncodeResponse(rw, response)
	}()

	alerts, err := models.DecodeAlertManagerAlerts(r.Body)
	if err != nil {
		klog.Errorf("fail to decode alerts: %v", err)
		metrics.RecordAPIParseFailure(source, "DecodeError")
		err = api.NewError(api.RequestParamError)
		response = api.CommonResponse{
			Code:    api.RequestParamError,
			Message: err.Error(),
		}
		return
	}
	klog.V(4).Infof("Received alertmanager alerts: %v", alerts)

	metrics.RecordAPIParseSuccess(source)

	for _, _alert := range alerts.Alerts {
		alert, err := _alert.ConvertAlertmanagerToCommonAlert()
		if err != nil {
			klog.Errorf("fail convert alertmanager alert: %v", err)
			metrics.RecordAPIParseFailure(source, "ConvertError")
			err = api.NewError(api.RequestParamError)
			response = api.CommonResponse{
				Code:    api.RequestParamError,
				Message: err.Error(),
			}
			continue
		}

		if err := callback(r.Context(), alert); err != nil {
			klog.Errorf("fail to callback alert %v: %v", alert, err)
			metrics.RecordCreateFailure(source)
			err = api.NewError(api.ServerError)
			response = api.CommonResponse{
				Code:    api.ServerError,
				Message: err.Error(),
			}
			continue
		}

		metrics.RecordCreateSuccess(source)
	}
}
