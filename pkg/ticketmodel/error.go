package ticketmodel

import (
	"errors"
	"os"
)

var (
	TicketAlreadyExistErr       = errors.New("AlreadyExist")
	TicketNotFoundErr           = errors.New("NotFound")
	TicketConclusionExistErr    = errors.New("ConclusionExist")
	TicketConclusionNotExistErr = errors.New("ConclusionNotExist")
	TicketWhySREExistErr        = errors.New("WhySREExist")
	TicketDiagnosisExistErr     = errors.New("DiagnosisExist")
)

const DefaultTicketSupervisorSRE = "syuan,cfeng,xfchen"

func GetTicketSupervisorSRE() string {
	if sre := os.Getenv("SRE"); sre != "" {
		return sre
	}
	return DefaultTicketSupervisorSRE
}
