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
	api.RegisterHandler("/default/alert", alert)
}

func alert(rw http.ResponseWriter, r *http.Request, callback func(ctx context.Context, alert *models.Alert) error,
	metrics *metrics.MetricsController,
) {
	source := string(models.DefaultAlertSource)

	response := api.CommonResponse{
		Code: api.OK,
	}
	defer func() {
		api.EncodeResponse(rw, response)
	}()

	alert, err := models.DecodeAlert(r.Body)
	if err != nil {
		klog.Errorf("fail to decode alert: %v", err)
		metrics.RecordAPIParseFailure(source, "DecodeError")
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
		metrics.RecordAPIParseFailure(source, "ValidationFailed")
		err = api.NewError(api.RequestParamError)
		response = api.CommonResponse{
			Code:    api.RequestParamError,
			Message: err.Error(),
		}
	}

	metrics.RecordAPIParseSuccess(source)

	if err := callback(r.Context(), alert); err != nil {
		klog.Errorf("fail to callback alert %v: %v", alert, err)
		metrics.RecordCreateFailure(source)
		err = api.NewError(api.ServerError)
		response = api.CommonResponse{
			Code:    api.ServerError,
			Message: err.Error(),
		}
	}

	metrics.RecordCreateSuccess(source)
}
