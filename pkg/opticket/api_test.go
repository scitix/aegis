package opticket

import (
	"context"

	// "strings"
	"testing"

	"github.com/scitix/aegis/tools"
)

func TestCreateTicket(t *testing.T) {
	client, err := CreateOpTicketClient(tools.GetOpEndpoint())
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = client.CreateTicket(context.Background(), "ap-southeast", "k8s", "odysseus-g20-011", "SGH402GB57", "aegis gpfs ib network mlx_5 issue", "this is just a test, diagnosis: dsadsad", "gpfs")
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestGetTicket(t *testing.T) {
	client, err := CreateOpTicketClient(tools.GetOpEndpoint())
	if err != nil {
		t.Fatalf("%v", err)
	}

	ticket, err := client.GetTicket(context.Background(), "t-20241030074914vEf")
	if err != nil {
		t.Fatalf("%v", err)
	}

	t.Logf("ticket: %v", ticket)
}

func TestDispatchTicket(t *testing.T) {
	client, err := CreateOpTicketClient(tools.GetOpEndpoint())
	if err != nil {
		t.Fatalf("%v", err)
	}

	// machine, err := client.GetMachineInfo(context.Background(), "test ", "cygnus170")
	// if err != nil {
	// 	t.Fatalf("%v", err)
	// }

	// t.Logf("machine: %v", machine)

	err = client.DispatchTicket(context.Background(), "t-20241030074914vEf", "aegis")
	if err != nil {
		t.Fatalf("err:  %v", err)
	}
}

func TestAcceptTicket(t *testing.T) {
	client, err := CreateOpTicketClient(tools.GetOpEndpoint())
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = client.AcceptTicket(context.Background(), "t-20241030074914vEf")
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestPatchTicket(t *testing.T) {
	client, err := CreateOpTicketClient(tools.GetOpEndpoint())
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = client.PatchTicket(context.Background(), "t-20241030074914vEf", "description", "update description test")
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestResolveTicket(t *testing.T) {
	client, err := CreateOpTicketClient(tools.GetOpEndpoint())
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = client.ResolveTicket(context.Background(), "t-20241030074914vEf", "selfhealing", "selfhealing", true)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

// func TestCloseTicket(t *testing.T) {
// 	client, err := CreateUcpClient(tools.GetUcpEndpoint())
// 	if err != nil {
// 		t.Fatalf("%v", err)
// 	}

// 	machine, err := client.GetMachineInfo(context.Background(), "odysseus", "odysseus-g20-011")
// 	if err != nil {
// 		t.Fatalf("%v", err)
// 	}

// 	t.Logf("machine: %v", machine)

// 	err = client.CloseTicket(context.Background(), machine.TicketId)
// 	if err != nil {
// 		t.Fatalf("%v", err)
// 	}
// }

// func TestCreateComponentTicket(t *testing.T) {
// 	client, err := CreateUcpClient(tools.GetUcpEndpoint())
// 	if err != nil {
// 		t.Fatalf("%v", err)
// 	}
// 	err = client.CreateComponentTicket(context.Background(), "odysseus", "odysseus-m-001", "aegis test", "this is just a test", "gpu/dcgm-exporter", "dcgm-exporter")
// 	if err != nil {
// 		t.Fatalf("%v", err)
// 	}
// }

func TestGetNodeFirstUnResolvedTicket(t *testing.T) {
	client, err := CreateOpTicketClient(tools.GetOpEndpoint())
	if err != nil {
		t.Fatalf("%v", err)
	}

	ticket, err := client.GetNodeFirstUnResovledTicket(context.Background(), "ap-southeast", "SGH402GB57")
	if err != nil {
		t.Fatalf("%v", err)
	}

	t.Logf("%v", ticket)
}

func TestListNodeTickets(t *testing.T) {
	client, err := CreateOpTicketClient(tools.GetOpEndpoint())
	if err != nil {
		t.Fatalf("%v", err)
	}

	ticket, err := client.ListNodeTickets(context.Background(), "ap-southeast", "SGH402GB57", 10)
	if err != nil {
		t.Fatalf("%v", err)
	}

	for _, tk := range ticket {
		t.Logf("%+v", tk)
	}
}

func TestNodeInfo(t *testing.T) {
	client, err := CreateOpTicketClient(tools.GetOpEndpoint())
	if err != nil {
		t.Fatalf("%v", err)
	}

	instance, err := client.GetNodeInfo(context.Background(), "ap-southeast", "k8s", "10.208.40.7")
	if err != nil {
		t.Fatalf("%v", err)
	}

	t.Logf("%+v", instance)
}
