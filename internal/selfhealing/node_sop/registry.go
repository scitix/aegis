package nodesop

import (
	"context"
	"errors"

	"github.com/scitix/aegis/internal/selfhealing/sop"
	"github.com/scitix/aegis/pkg/prom"
)

var registryMap map[string]SOP = make(map[string]SOP)

func RegisterSOP(name string, sop SOP) error {
	if _, ok := registryMap[name]; ok {
		return errors.New("Already registered")
	}

	registryMap[name] = sop
	return nil
}

func GetSOP(name string) (SOP, error) {
	if sop, ok := registryMap[name]; ok {
		return sop, nil
	}

	return nil, errors.New("Not Found")
}

// CordonSOP is implemented by SOPs that need to control cordon behavior.
// SOPs that do not implement this interface are assumed to require cordon.
type CordonSOP interface {
	NeedCordon(ctx context.Context, node string, status *prom.AegisNodeStatus) bool
}

// PreemptableSOP is implemented by SOPs whose active SRE-owned ticket may be
// deleted and superseded when a new SOP needs to run on the same node.
// SOPs that do not implement this interface are not preemptable.
type PreemptableSOP interface {
	IsPreemptable() bool
}

type SOP interface {
	CreateInstance(ctx context.Context, bridge *sop.ApiBridge) error

	// 真实性评估
	Evaluate(ctx context.Context, node string, status *prom.AegisNodeStatus) bool

	// 是否会创建工单
	// KickTicket(ctx context.Context, node string, status *prom.AegisNodeStatus) bool

	// 执行 SOP
	Execute(ctx context.Context, node string, status *prom.AegisNodeStatus) error

	// 在有工单状态下，执行非自愈的清理动作
	Cleanup(ctx context.Context, node string, status *prom.AegisNodeStatus) error
}
