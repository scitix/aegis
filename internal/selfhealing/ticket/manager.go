package ticket

import (
	"context"
	"fmt"

	"github.com/scitix/aegis/pkg/nodeticket"
	"github.com/scitix/aegis/pkg/opticket"
	"github.com/scitix/aegis/pkg/ticketmodel"
	"github.com/scitix/aegis/pkg/uticket"
)

type TicketSystem string

const (
	TicketSystemNode   TicketSystem = "Node"
	TicketSystemScitix TicketSystem = "Scitix"
	TicketSystemUcp    TicketSystem = "UCP"
)

func NewTicketManagerBySystem(ctx context.Context, system TicketSystem, args *ticketmodel.TicketManagerArgs) (ticketmodel.TicketManagerInterface, error) {
	switch system {
	case TicketSystemNode:
		return nodeticket.NewNodeTicketManager(ctx, args)
	case TicketSystemScitix:
		return opticket.NewOPTicketManager(ctx, args)
	case TicketSystemUcp:
		return uticket.NewTicketManager(ctx, args)
	default:
		return nil, fmt.Errorf("unsupported ticket system: %s", system)
	}
}
