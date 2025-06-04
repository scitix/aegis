package apis

import (
	"context"
	"net/http"

	"gitlab.scitix-inner.ai/k8s/aegis/api"
	"gitlab.scitix-inner.ai/k8s/aegis/api/models"
	"k8s.io/klog/v2"
)

func init() {
	api.RegisterHandler("/default/alert", alert)
}

func alert(rw http.ResponseWriter, r *http.Request, callback func(ctx context.Context, alert *models.Alert) error) {
	var response = api.CommonResponse{
		Code: api.OK,
	}
	defer func() {
		api.EncodeResponse(rw, response)
	}()

	alert, err := models.DecodeAlert(r.Body)
	if err != nil {
		klog.Errorf("fail to decode alert: %v", err)
		err = api.NewError(api.RequestParamError)
		response = api.CommonResponse{
			Code:    api.RequestParamError,
			Message: err.Error(),
		}
		return
	}
	klog.V(4).Infof("Recived alert: %s", alert)

	if err := alert.Validate(); err != nil {
		klog.Errorf("invalid alert: %v", err)
		err = api.NewError(api.RequestParamError)
		response = api.CommonResponse{
			Code:    api.RequestParamError,
			Message: err.Error(),
		}
	}

	if err := callback(r.Context(), alert); err != nil {
		klog.Errorf("fail to callback alert %v: %v", alert, err)
		err = api.NewError(api.ServerError)
		response = api.CommonResponse{
			Code:    api.ServerError,
			Message: err.Error(),
		}
	}
}
