package analyzer

import (
	"gitlab.scitix-inner.ai/k8s/aegis/pkg/analyzer/common"
)

var customAnalyzerMap = map[string]common.IAnalyzer{}

func GetAnalyzerMap() map[string]common.IAnalyzer {
	if _, ok := customAnalyzerMap["Pod"]; !ok {
		customAnalyzerMap["Pod"] = NewPodAnalyzer()
	}

	if _, ok := customAnalyzerMap["Node"]; !ok {
		customAnalyzerMap["Node"] = NewNodeAnalyzer()
	}

	mergedAnalyzerMap := make(map[string]common.IAnalyzer)

	// add core analyzer
	for key, value := range customAnalyzerMap {
		mergedAnalyzerMap[key] = value
	}

	return mergedAnalyzerMap
}
