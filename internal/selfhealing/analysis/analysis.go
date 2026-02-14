package analysis

import (
	"bufio"
	"fmt"
	"io"
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

// ConditionConfig holds the full configuration for a single fault condition.
type ConditionConfig struct {
	Priority     Priority
	AffectsLoad  bool
	DeviceIDMode string // "all" / "index" / "mask" / "id" / "-"
}

// ParseConditionConfig parses the four-column format
// "Condition:Priority:AffectsLoad:DeviceIDMode" (third and fourth columns are
// optional and default to false and "-" respectively for backward compatibility).
// Lines starting with # and blank lines are ignored.
func ParseConditionConfig(content string) (map[string]ConditionConfig, error) {
	result := make(map[string]ConditionConfig)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		strs := strings.Split(line, ":")
		if len(strs) < 2 || len(strs) > 4 {
			return nil, fmt.Errorf("invalid config format: %s", line)
		}

		pri, err := strconv.Atoi(strings.TrimSpace(strs[1]))
		if err != nil {
			return nil, fmt.Errorf("error conv priority %q: %s", strs[1], err)
		}

		cfg := ConditionConfig{
			Priority:     Priority(pri),
			AffectsLoad:  false,
			DeviceIDMode: "-",
		}

		if len(strs) >= 3 {
			cfg.AffectsLoad, err = strconv.ParseBool(strings.TrimSpace(strs[2]))
			if err != nil {
				return nil, fmt.Errorf("error conv AffectsLoad %q: %s", strs[2], err)
			}
		}
		if len(strs) == 4 {
			cfg.DeviceIDMode = strings.TrimSpace(strs[3])
		}

		result[strings.TrimSpace(strs[0])] = cfg
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ParsePriorityConfig parses priority config content and returns a
// Condition→Priority map. It accepts both the legacy two-column format and the
// extended four-column format; the extra columns are silently ignored here.
func ParsePriorityConfig(content string) (map[string]Priority, error) {
	full, err := ParseConditionConfig(content)
	if err != nil {
		return nil, err
	}
	result := make(map[string]Priority, len(full))
	for k, v := range full {
		result[k] = v.Priority
	}
	return result, nil
}

func InitAnalysisConfig(config string) error {
	file, err := os.Open(config)
	if err != nil {
		return err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	parsed, err := ParsePriorityConfig(string(content))
	if err != nil {
		return err
	}
	for k, v := range parsed {
		nodeOperateConfig[k] = v
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
