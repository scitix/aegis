package prom

import (
	"context"
	"fmt"
	"strconv"

	"github.com/prometheus/common/model"
	"k8s.io/klog/v2"
)

type AegisNodeStatus struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Condition string `json:"condition"`
	ID        string `json:"id"`
	PciBdf    string `json:"pci_bdf"`
	Msg       string `json:"msg"`
	Value     int    `json:"value"`
}
type NodeGpuStatus struct {
	ID            string  `json:"id"`
	Mode          string  `json:"mode"`
	PodNamespace  string  `json:"podNamespace"`
	PodName       string  `json:"podName"`
	ContainerName string  `json:"containerName"`
	Util          float64 `json:"util"`
}

func (p *PromAPI) GetNodeStatuses(ctx context.Context, node, tpe string) ([]AegisNodeStatus, error) {
	query := fmt.Sprintf("aegis_node_status_condition{node=\"%s\"}", node)
	if tpe != "" {
		query = fmt.Sprintf("aegis_node_status_condition{node=\"%s\", type=\"%s\"}", node, tpe)
	}
	value, err := p.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	return resolveNodesStatus(value), nil
}

func (p *PromAPI) GetNodeGpuStatuses(ctx context.Context, node string) ([]NodeGpuStatus, error) {
	query := fmt.Sprintf("DCGM_FI_DEV_DEC_UTIL{Hostname=\"%s\"}", node)
	value, err := p.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	return resolveGpu(value), nil
}

func (p *PromAPI) ListNodeStatusesWithCondition(ctx context.Context, condition string) ([]AegisNodeStatus, error) {
	query := fmt.Sprintf("count by (node) (aegis_node_status_condition{condition=\"%s\"})", condition)
	value, err := p.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	return resolveNodesStatus(value), nil
}

func (p *PromAPI) ListNodeStatusesWithQuery(ctx context.Context, query string) ([]AegisNodeStatus, error) {
	value, err := p.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	return resolveNodesStatus(value), nil
}

func (p *PromAPI) ListNodesWithQuery(ctx context.Context, query string) ([]string, error) {
	value, err := p.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	return resolveNodes(value), nil
}

func resolveNodes(value model.Value) []string {
	nodes := make([]string, 0)

	switch value.Type() {
	case model.ValVector:
		for _, val := range value.(model.Vector) {
			metric := val.Metric
			nodes = append(nodes, string(metric["node"]))
		}
	default:
		return nil
	}

	return nodes
}

func resolveNodesStatus(value model.Value) []AegisNodeStatus {
	nodes := make([]AegisNodeStatus, 0)

	switch value.Type() {
	case model.ValVector:
		for _, val := range value.(model.Vector) {
			metric := val.Metric
			condition := string(metric["condition"])
			node := string(metric["node"])
			ty := string(metric["type"])
			id := string(metric["id"])
			pcibdf := string(metric["pci_bdf"])
			msg := string(metric["msg"])
			count := int(val.Value)

			code := string(metric["code"]) // replace count if code exists
			if code != "" {
				c, err := strconv.Atoi(code)
				if err != nil {
					klog.Errorf("error conv %s to int: %s", code, err)
				} else {
					count = c
				}
			}

			nodes = append(nodes, AegisNodeStatus{
				Name:      node,
				Type:      ty,
				Condition: condition,
				ID:        id,
				PciBdf:    pcibdf,
				Msg:       msg,
				Value:     count,
			})
		}
	default:
		return nil
	}

	return nodes
}

func resolveGpu(value model.Value) []NodeGpuStatus {
	gpus := make([]NodeGpuStatus, 0)
	switch value.Type() {
	case model.ValVector:
		for _, val := range value.(model.Vector) {
			metric := val.Metric
			gpus = append(gpus, NodeGpuStatus{
				ID:            string(metric["gpu"]),
				Mode:          string(metric["modelName"]),
				PodNamespace:  string(metric["exported_namespace"]),
				PodName:       string(metric["exported_pod"]),
				ContainerName: string(metric["exported_container"]),
				Util:          float64(val.Value),
			})
		}
	default:
		return nil
	}

	return gpus
}
