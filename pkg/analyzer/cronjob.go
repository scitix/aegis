package analyzer

import (
	"fmt"

	kanalyzer "github.com/k8sgpt-ai/k8sgpt/pkg/analyzer"
	"github.com/scitix/aegis/pkg/analyzer/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CronJobAnalyzer struct {
	kanalyzer.CronJobAnalyzer
}

func NewCronJobAnalyzer() CronJobAnalyzer {
	return CronJobAnalyzer{
		kanalyzer.CronJobAnalyzer{},
	}
}

func (analyzer CronJobAnalyzer) Analyze(a common.Analyzer) (*common.Result, error) {
	cronJob, err := a.Client.GetClient().BatchV1().CronJobs(a.Namespace).Get(a.Context, a.Name, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	a.LabelSelector = v1.FormatLabelSelector(&v1.LabelSelector{
		MatchLabels: cronJob.Labels,
	})

	results, err := analyzer.CronJobAnalyzer.Analyze(a.Analyzer)
	if err != nil {
		return nil, err
	}

	if len(results) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %d", len(results))
	}

	result := &common.Result{}
	result.Result = results[0]

	return result, nil
}
