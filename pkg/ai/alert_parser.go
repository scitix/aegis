package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	kai "github.com/k8sgpt-ai/k8sgpt/pkg/ai"
	"gitlab.scitix-inner.ai/k8s/aegis/api/models"
)

type AlertParser interface {
	Parse(ctx context.Context, raw []byte) ([]*models.Alert, error)
}

type DefaultAIAlertParser struct {
	client kai.IAI
}

func NewDefaultAIAlertParserWithClient(client kai.IAI) *DefaultAIAlertParser {
	return &DefaultAIAlertParser{client: client}
}

func (p *DefaultAIAlertParser) Parse(ctx context.Context, raw []byte) ([]*models.Alert, error) {
	prompt, err := GetRenderedPrompt("AlertParse", PromptData{RawAlert: string(raw)})
	if err != nil {
		return nil, fmt.Errorf("render prompt: %w", err)
	}

	resp, err := p.client.GetCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm call failed: %w", err)
	}

	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```") {
		resp = strings.TrimPrefix(resp, "```json")
		resp = strings.TrimPrefix(resp, "```")
		resp = strings.TrimSuffix(resp, "```")
		resp = strings.TrimSpace(resp)
	}

	// support for single/multi alert
	var alertList []*models.Alert
	if strings.HasPrefix(resp, "[") {
		// multi alert
		if err := json.Unmarshal([]byte(resp), &alertList); err != nil {
			return nil, fmt.Errorf("llm response (list) not valid JSON: %w", err)
		}
	} else {
		// single alert
		var alert models.Alert
		if err := json.Unmarshal([]byte(resp), &alert); err != nil {
			return nil, fmt.Errorf("llm response not valid JSON: %w", err)
		}
		alertList = []*models.Alert{&alert}
	}

	for _, a := range alertList {
		if a.FingerPrint == "" {
			fingerprint, err := generateFingerprint(a)
			if err != nil {
				return nil, fmt.Errorf("failed to generate fingerprint: %w", err)
			}
			a.FingerPrint = fingerprint
		}
		a.AlertSourceType = models.AIAlertSource
		if err := a.Validate(); err != nil {
			return nil, fmt.Errorf("alert validation failed: %w", err)
		}
	}

	return alertList, nil
}

func generateFingerprint(alert *models.Alert) (string, error) {
	if len(alert.Details) == 0 {
		return "", fmt.Errorf("alert details are empty, cannot generate fingerprint")
	}

	labelNames := make([]string, 0, len(alert.Details))
	for labelName := range alert.Details {
		labelNames = append(labelNames, labelName)
	}
	sort.Strings(labelNames)

	var builder strings.Builder
	for _, labelName := range labelNames {
		if alert.Details[labelName] == "" {
			continue
		}
		builder.WriteString(labelName)
		builder.WriteString(alert.Details[labelName])
		builder.WriteString(":")
	}

	data := builder.String()
	if data == "" {
		return "", fmt.Errorf("no valid data to generate fingerprint")
	}

	hash := sha256.New()
	hash.Write([]byte(data))
	fingerprint := hex.EncodeToString(hash.Sum(nil))

	if len(fingerprint) > 63 {
		fingerprint = fingerprint[:63]
	}

	return fingerprint, nil
}
