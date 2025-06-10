package models

import (
	"encoding/json"
	"fmt"
	"io"
)

type Alert struct {
	AlertSourceType AlertSourceType
	Type            string              `json:"type"`
	Status          string              `json:"status"`
	InvolvedObject  AlertInvolvedObject `json:"involvedObject"`
	Details         map[string]string   `json:"details"`
	FingerPrint     string              `json:"fingerprint"`
}

type AlertInvolvedObject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Node      string `json:"node"`
}

func (a *Alert) validateType() error {
	if a.Type == "" {
		return fmt.Errorf("empty type")
	}
	return nil
}

func (a *Alert) validateStatus() error {
	if (a.Status != AlertStatusFiring) && (a.Status != AlertStatusResolved) {
		return fmt.Errorf("invalid alert status: %v", a.Status)
	}
	return nil
}

func (a *Alert) validateObject() error {
	if validateKind(a.InvolvedObject.Kind) != nil {
		return fmt.Errorf("invalid alert involved object kind: %v", a.InvolvedObject.Kind)
	}

	if a.InvolvedObject.Name == "" {
		return fmt.Errorf("empty alert involved object name")
	}

	if a.InvolvedObject.Kind == PodKind && a.InvolvedObject.Namespace == "" {
		return fmt.Errorf("empty alert involved object namespace")
	}
	return nil
}

func (a *Alert) validateFingerPrint() error {
	if a.FingerPrint == "" {
		return fmt.Errorf("empty fingerprint")
	}

	return nil
}

func (a *Alert) Validate() error {
	var err error
	if err = a.validateType(); err != nil {
		return err
	}

	if err = a.validateStatus(); err != nil {
		return err
	}

	if err = a.validateObject(); err != nil {
		return err
	}

	if err = a.validateFingerPrint(); err != nil {
		return err
	}

	return nil
}

func DecodeAlert(r io.ReadCloser) (*Alert, error) {
	defer r.Close()

	alert := &Alert{
		Details: make(map[string]string),
	}
	err := json.NewDecoder(r).Decode(alert)
	alert.AlertSourceType = DefaultAlertSource
	return alert, err
}
