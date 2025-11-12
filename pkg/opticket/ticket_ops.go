package opticket

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/scitix/aegis/pkg/ticketmodel"
	"k8s.io/klog/v2"
)

func (t *OpTicketManager) GetRootCauseDescription(ctx context.Context) (*ticketmodel.TicketCause, error) {
	if t.ticket == nil {
		return nil, ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err := description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return nil, fmt.Errorf("error unmarshal description: %s", err)
	}

	return &description.Cause, nil
}

func (t *OpTicketManager) AddRootCauseDescription(ctx context.Context, cause string, condition interface{}) (count int, err error) {
	defer func() {
		if err != nil {
			klog.Errorf("error add root cause description: %s", err)
		}
	}()

	if t.ticket == nil {
		return 0, ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err = description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return 0, fmt.Errorf("error unmarshal description: %s", err)
	}

	if description.Conclusion != "" {
		return 0, ticketmodel.TicketConclusionExistErr
	}

	if len(description.Workflows) > 0 {
		return 0, errors.New("cannot update cause after workflow begin")
	}

	if description.Cause.Cause != "" && description.Cause.Cause != cause {
		return 0, errors.New("different cause")
	}

	description.Cause = ticketmodel.TicketCause{
		Timestamps: time.Now(),
		Cause:      cause,
		Condition:  condition,
		Count:      description.Cause.Count + 1,
	}

	c, err := description.Marshal()
	if err != nil {
		return 0, fmt.Errorf("error marshal description: %s", err)
	}

	err = t.u.PatchTicket(ctx, t.ticket.TicketId, "description", string(c))
	if err != nil {
		return 0, fmt.Errorf("error update ticket description: %s", err)
	}

	t.ticket.Description = string(c)

	return int(description.Cause.Count), nil
}

func (t *OpTicketManager) AddOrUpdateRootCauseDescription(ctx context.Context, cause string, condition interface{}) (updated bool, err error) {
	if t.ticket == nil {
		return false, ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err = description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return false, fmt.Errorf("error unmarshal description: %s", err)
	}

	if description.Conclusion != "" {
		return false, ticketmodel.TicketConclusionExistErr
	}

	if len(description.Workflows) > 0 {
		return false, errors.New("cannot update cause after workflow begin")
	}

	if description.Cause.Cause != "" && description.Cause.Cause != cause {
		description.Cause = ticketmodel.TicketCause{
			Timestamps: time.Now(),
			Cause:      cause,
			Condition:  condition,
			Count:      1,
		}
	} else {
		description.Cause = ticketmodel.TicketCause{
			Timestamps: time.Now(),
			Cause:      cause,
			Condition:  condition,
			Count:      description.Cause.Count + 1,
		}
		updated = true
	}

	c, err := description.Marshal()
	if err != nil {
		return false, fmt.Errorf("error marshal description: %s", err)
	}

	err = t.u.PatchTicket(ctx, t.ticket.TicketId, "description", string(c))
	if err != nil {
		return false, fmt.Errorf("error update ticket description: %s", err)
	}
	t.ticket.Description = string(c)

	return updated, nil
}

func (t *OpTicketManager) GetActionCount(ctx context.Context, action ticketmodel.TicketWorkflowAction) (int, error) {
	if t.ticket == nil {
		return 0, ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err := description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return 0, fmt.Errorf("error unmarshal description: %s", err)
	}

	count := 0
	for _, w := range description.Workflows {
		if w.Action == action {
			count++
		}
	}

	return count, nil
}

func (t *OpTicketManager) AddConclusion(ctx context.Context, conclusion string) (err error) {
	if t.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	defer func() {
		if err != nil {
			klog.Errorf("error add conclusion: %s", err)
		}
	}()

	var description ticketmodel.TicketDescription
	err = description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return fmt.Errorf("error unmarshal description: %s", err)
	}

	if description.Conclusion != "" {
		return ticketmodel.TicketConclusionExistErr
	}

	description.Conclusion = conclusion
	c, err := description.Marshal()
	if err != nil {
		return fmt.Errorf("error marshal description: %s", err)
	}

	err = t.u.PatchTicket(ctx, t.ticket.TicketId, "description", string(c))
	if err != nil {
		return fmt.Errorf("error update ticket description: %s", err)
	}
	t.ticket.Description = string(c)
	return nil
}

func (t *OpTicketManager) AddDiagnosis(ctx context.Context, diagnosis []ticketmodel.Diagnose) (err error) {
	defer func() {
		if err != nil {
			klog.Errorf("error add diagnosis: %s", err)
		}
	}()

	if t.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err = description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return fmt.Errorf("error unmarshal description: %s", err)
	}

	if description.Diagnosis != nil {
		return ticketmodel.TicketDiagnosisExistErr
	}

	description.Diagnosis = diagnosis
	c, err := description.Marshal()
	if err != nil {
		return fmt.Errorf("error marshal description: %s", err)
	}

	err = t.u.PatchTicket(ctx, t.ticket.TicketId, "description", string(c))
	if err != nil {
		return fmt.Errorf("error update ticket description: %s", err)
	}
	t.ticket.Description = string(c)
	return nil
}

func (t *OpTicketManager) AddWhySRE(ctx context.Context, whySRE string) (err error) {
	defer func() {
		if err != nil {
			klog.Errorf("error add whySRE: %s", err)
		}
	}()

	if t.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err = description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return fmt.Errorf("error unmarshal description: %s", err)
	}

	if description.WhySRE != "" {
		return ticketmodel.TicketWhySREExistErr
	}

	description.WhySRE = whySRE
	c, err := description.Marshal()
	if err != nil {
		return fmt.Errorf("error marshal description: %s", err)
	}

	err = t.u.PatchTicket(ctx, t.ticket.TicketId, "description", string(c))
	if err != nil {
		return fmt.Errorf("error update ticket description: %s", err)
	}
	t.ticket.Description = string(c)
	return nil
}

func (t *OpTicketManager) GetWorkflows(ctx context.Context) ([]ticketmodel.TicketWorkflow, error) {
	if t.ticket == nil {
		return nil, ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err := description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return nil, fmt.Errorf("error unmarshal description: %s", err)
	}

	return description.Workflows, nil
}

func (t *OpTicketManager) GetLastWorkflow(ctx context.Context) (*ticketmodel.TicketWorkflow, error) {
	if t.ticket == nil {
		return nil, ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err := description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return nil, fmt.Errorf("error unmarshal description: %s", err)
	}

	if l := len(description.Workflows); l == 0 {
		return nil, nil
	} else {
		return &description.Workflows[l-1], nil
	}
}

func (t *OpTicketManager) AddWorkflow(ctx context.Context, action ticketmodel.TicketWorkflowAction, status ticketmodel.TicketWorkflowStatus, message *string) error {
	if t.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err := description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return fmt.Errorf("error unmarshal description: %s", err)
	}

	if description.Conclusion != "" {
		return ticketmodel.TicketConclusionExistErr
	}

	if description.Workflows == nil {
		description.Workflows = make([]ticketmodel.TicketWorkflow, 0)
	}

	description.Workflows = append(description.Workflows, ticketmodel.TicketWorkflow{
		Timestamps: time.Now(),
		Action:     action,
		Status:     status,
		Message:    message,
	})

	c, err := description.Marshal()
	if err != nil {
		return fmt.Errorf("error marshal description: %s", err)
	}

	err = t.u.PatchTicket(ctx, t.ticket.TicketId, "description", string(c))
	if err != nil {
		return fmt.Errorf("error update ticket description: %s", err)
	}

	t.ticket.Description = string(c)
	return nil
}

func (t *OpTicketManager) UpdateWorkflow(ctx context.Context, action ticketmodel.TicketWorkflowAction, status ticketmodel.TicketWorkflowStatus, message *string) error {
	if t.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err := description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return fmt.Errorf("error unmarshal description: %s", err)
	}

	if description.Conclusion != "" {
		return ticketmodel.TicketConclusionExistErr
	}

	l := len(description.Workflows)

	if l == 0 || description.Workflows[l-1].Action != action {
		return fmt.Errorf("action %s not found", action)
	}

	description.Workflows[l-1] = ticketmodel.TicketWorkflow{
		Timestamps: description.Workflows[l-1].Timestamps,
		Action:     action,
		Status:     status,
		Message:    message,
	}

	c, err := description.Marshal()
	if err != nil {
		return fmt.Errorf("error marshal description: %s", err)
	}

	err = t.u.PatchTicket(ctx, t.ticket.TicketId, "description", string(c))
	if err != nil {
		return fmt.Errorf("error update ticket description: %s", err)
	}
	t.ticket.Description = string(c)
	return nil
}

func (t *OpTicketManager) AddShutdownDescription(ctx context.Context, status ticketmodel.TicketWorkflowStatus, message *string) error {
	if t.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err := description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return fmt.Errorf("error unmarshal description: %s", err)
	}

	description.ShutDown = &ticketmodel.TicketWorkflow{
		Timestamps: time.Now(),
		Action:     ticketmodel.TicketWorkflowActionShutdown,
		Status:     status,
		Message:    message,
	}

	c, err := description.Marshal()
	if err != nil {
		return fmt.Errorf("error marshal description: %s", err)
	}

	err = t.u.PatchTicket(ctx, t.ticket.TicketId, "description", string(c))
	if err != nil {
		return fmt.Errorf("error update ticket description: %s", err)
	}

	t.ticket.Description = string(c)
	return nil
}

func (t *OpTicketManager) UpdateShutdownDescription(ctx context.Context, status ticketmodel.TicketWorkflowStatus, message *string) error {
	if t.ticket == nil {
		return ticketmodel.TicketNotFoundErr
	}

	var description ticketmodel.TicketDescription
	err := description.Unmarshal([]byte(t.ticket.Description))
	if err != nil {
		return fmt.Errorf("error unmarshal description: %s", err)
	}

	description.ShutDown = &ticketmodel.TicketWorkflow{
		Timestamps: description.ShutDown.Timestamps,
		Action:     ticketmodel.TicketWorkflowActionShutdown,
		Status:     status,
		Message:    message,
	}

	c, err := description.Marshal()
	if err != nil {
		return fmt.Errorf("error marshal description: %s", err)
	}

	err = t.u.PatchTicket(ctx, t.ticket.TicketId, "description", string(c))
	if err != nil {
		return fmt.Errorf("error update ticket description: %s", err)
	}
	t.ticket.Description = string(c)
	return nil
}
