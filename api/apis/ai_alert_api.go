package apis

import (
	"context"
	"io"
	"net/http"

	"gitlab.scitix-inner.ai/k8s/aegis/api"
	"gitlab.scitix-inner.ai/k8s/aegis/api/models"
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/ai"
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/metrics"
	"k8s.io/klog/v2"
)

var aiAlertParser ai.AlertParser

func init() {
	// 延时注入AIclient
	api.RegisterHandler("/ai/alert", aiAlertHandler)
}

func SetAIAlertParser(parser ai.AlertParser) {
	aiAlertParser = parser
}

// aiAlertHandler: 接收原始告警 JSON（任意系统），交由 LLM 解析成 models.Alert 结构
func aiAlertHandler(rw http.ResponseWriter, r *http.Request, callback func(ctx context.Context, alert *models.Alert) error,
	metrics *metrics.MetricsController,
) {
	source := "ai"

	response := api.CommonResponse{
		Code: api.OK,
	}
	defer func() {
		api.EncodeResponse(rw, response)
	}()

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		klog.Errorf("failed to read request body: %v", err)
		metrics.RecordAPIParseFailure(source, "ReadError")
		response = api.CommonResponse{
			Code:    api.RequestParamError,
			Message: "无法读取请求体",
		}
		return
	}

	if aiAlertParser == nil {
        klog.Warningf("AIAlertParser not configured, skip AI parsing")
        metrics.RecordAPIParseFailure(source, "AIAlertParserNotConfigured")
        response = api.CommonResponse{
            Code:    api.RequestParamError,
            Message: "AIAlertParser 未配置，无法解析告警",
        }
        return
    }

	alerts, err := aiAlertParser.Parse(r.Context(), raw)
	if err != nil {
		klog.Errorf("failed to parse alert via AI: %v", err)
		metrics.RecordAPIParseFailure(source, "AIParseError")
		response = api.CommonResponse{
			Code:    api.RequestParamError,
			Message: "AI解析失败: " + err.Error(),
		}
		return
	}

	// klog.V(4).Infof("Parsed alert from AI: %+v", alert)

	metrics.RecordAPIParseSuccess(source)

	for _, alert := range alerts {
		if err := callback(r.Context(), alert); err != nil {
			klog.Errorf("failed to callback alert: %v", err)
			metrics.RecordCreateFailure(source)
			response = api.CommonResponse{
				Code:    api.ServerError,
				Message: "告警处理失败: " + err.Error(),
			}
			return
		}
		metrics.RecordCreateSuccess(source)
	}
}

