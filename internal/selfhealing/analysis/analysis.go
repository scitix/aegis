package analysis

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/scitix/aegis/pkg/prom"
)

type Priority int

const (
	NodeNotReady Priority = 0
	NodeCordon   Priority = 1
	Emergency    Priority = 99
	CanIgnore    Priority = 999
	MustIgnore   Priority = 9999
)

type NodeStatusAnalysisResult struct {
	NotReady       *prom.AegisNodeStatus
	Cordon         *prom.AegisNodeStatus
	EmergencyList  []prom.AegisNodeStatus
	CanIgnoreList  []prom.AegisNodeStatus
	MustIgnoreList []prom.AegisNodeStatus
}

var nodeOperateConfig map[string]Priority = make(map[string]Priority)

func InitAnalysisConfig(config string) error {
	file, err := os.Open(config)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}

		strs := strings.Split(line, ":")
		if len(strs) != 2 {
			return fmt.Errorf("Invalid config format: %s", line)
		}

		pri, err := strconv.Atoi(strs[1])
		if err != nil {
			return fmt.Errorf("Error conv string %s to int: %s", strs[1], err)
		}
		nodeOperateConfig[strs[0]] = Priority(pri)
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

type nodestatus struct {
	priority Priority
	status   prom.AegisNodeStatus
}

type nodestatuslist []nodestatus

func (el nodestatuslist) Len() int {
	return len(el)
}

func (el nodestatuslist) Less(i, j int) bool {
	return el[i].priority <= el[j].priority
}

func (el nodestatuslist) Swap(i, j int) {
	el[i], el[j] = el[j], el[i]
}

func AnalysisNodeStatus(nodeStatus []prom.AegisNodeStatus) *NodeStatusAnalysisResult {
	result := &NodeStatusAnalysisResult{
		NotReady:       nil,
		Cordon:         nil,
		EmergencyList:  make([]prom.AegisNodeStatus, 0),
		CanIgnoreList:  make([]prom.AegisNodeStatus, 0),
		MustIgnoreList: make([]prom.AegisNodeStatus, 0),
	}

	es := make([]nodestatus, 0)
	cs := make([]nodestatus, 0)
	ms := make([]nodestatus, 0)

	for _, s := range nodeStatus {
		status := s

		condition := status.Condition
		priority, ok := nodeOperateConfig[condition]
		if !ok {
			priority = CanIgnore
		}

		switch {
		case priority == NodeNotReady:
			result.NotReady = &status
		case priority == NodeCordon:
			result.Cordon = &status
		case priority <= Emergency && priority > 1:
			es = append(es, nodestatus{priority: priority, status: status})
		case priority <= CanIgnore && priority > Emergency:
			cs = append(cs, nodestatus{priority: priority, status: status})
		case priority > CanIgnore:
			ms = append(ms, nodestatus{priority: priority, status: status})
		}
	}

	sort.Sort(nodestatuslist(es))
	sort.Sort(nodestatuslist(cs))
	sort.Sort(nodestatuslist(ms))

	for _, status := range es {
		result.EmergencyList = append(result.EmergencyList, status.status)
	}

	for _, status := range cs {
		result.CanIgnoreList = append(result.CanIgnoreList, status.status)
	}

	for _, status := range ms {
		result.MustIgnoreList = append(result.MustIgnoreList, status.status)
	}

	return result
}
