package apis

import (
	"context"
	"net/http"

	"gitlab.scitix-inner.ai/k8s/aegis/api"
	"gitlab.scitix-inner.ai/k8s/aegis/api/models"
	"k8s.io/klog/v2"
)

func init() {
	api.RegisterHandler("/alertmanager/alert", alertmanager)
}

func alertmanager(rw http.ResponseWriter, r *http.Request, callback func(ctx context.Context, alert *models.Alert) error) {
	var response = api.CommonResponse{
		Code: api.OK,
	}
	defer func() {
		api.EncodeResponse(rw, response)
	}()

	alerts, err := models.DecodeAlertManagerAlerts(r.Body)
	if err != nil {
		klog.Errorf("fail to decode alerts: %v", err)
		err = api.NewError(api.RequestParamError)
		response = api.CommonResponse{
			Code:    api.RequestParamError,
			Message: err.Error(),
		}
		return
	}
	klog.V(4).Infof("Received alertmanager alerts: %v", alerts)

	for _, _alert := range alerts.Alerts {
		if alert, err := _alert.ConvertAlertManagerToCommonAlert(); err != nil {
			klog.Errorf("fail convert alertmanager alert: %v", err)
			err = api.NewError(api.RequestParamError)
			response = api.CommonResponse{
				Code:    api.RequestParamError,
				Message: err.Error(),
			}
		} else if err := callback(r.Context(), alert); err != nil {
			klog.Errorf("fail to callback alert %v: %v", alert, err)
			err = api.NewError(api.ServerError)
			response = api.CommonResponse{
				Code:    api.ServerError,
				Message: err.Error(),
			}
		}
	}
}
