package models

import (
	"fmt"
	"io"
	"strings"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog/v2"
)

type AlertManagerAlert struct {
	// ends at
	// Required: true
	// Format: date-time
	EndsAt *strfmt.DateTime `json:"endsAt"`

	// starts at
	// Required: true
	// Format: date-time
	StartsAt *strfmt.DateTime `json:"startsAt"`

	// status
	// Required: true
	Status string `json:"status"`

	// updated at
	// Required: true
	// Format: date-time
	UpdatedAt *strfmt.DateTime `json:"updatedAt"`

	Labels map[string]string `json:"labels"`

	Annotations map[string]string `json:"annotations"`

	FingerPrint string `json:"fingerprint"`
}

type AlertManagerAlerts struct {
	// status
	// Required: true
	Status string `json:"status"`

	Alerts []AlertManagerAlert `json:"alerts"`

	CommonAnnotations map[string]string `json:"commonAnnotations"`
}

func (_alert *AlertManagerAlert) ConvertAlertManagerToCommonAlert() (*Alert, error) {
	if _alert == nil {
		return nil, nil
	}

	alert := &Alert{
		AlertSourceType: AlertManagerAlertSource,
		Details:         _alert.Labels,
		FingerPrint:     _alert.FingerPrint,
	}

	if description, ok := _alert.Annotations["description"]; ok {
		alert.Details["description"] = description
	}

	if len(alert.FingerPrint) == 0 {
		alert.FingerPrint = uuid.New().String()
		klog.Warningf("fingerprint not found, random generate an uuid: %s", alert.FingerPrint)
	}

	switch strings.ToLower(_alert.Status) {
	case strings.ToLower(AlertStatusFiring):
		alert.Status = AlertStatusFiring
	case strings.ToLower(AlertStatusResolved):
		alert.Status = AlertStatusResolved
	default:
		return nil, fmt.Errorf("invalid status: %v", _alert.Status)
	}

	labels := _alert.Labels
	if alertname, ok := labels["alertname"]; ok {
		alert.Type = alertname
	} else {
		return nil, fmt.Errorf("empty alert type")
	}

	kind, ok := labels["kind"]
	if !ok {
		return nil, fmt.Errorf("label kind requried")
	}
	if err := validateKind(kind); err != nil {
		return nil, err
	}

	// event alert
	objectName, objectnameOk := labels["involved_object_name"]
	namespace := labels["namespace"]
	if objectnameOk {
		alert.InvolvedObject = AlertInvolvedObject{
			Kind:      kind,
			Name:      objectName,
			Namespace: namespace,
		}
	} else {
		// metrics alert
		node, nodeOk := labels["node"]
		instance, instanceOk := labels["instance"]
		if instanceOk && !nodeOk {
			node = instance
		}

		pod := labels["pod"]
		job := labels["job"]
		if kind == PodKind {
			alert.InvolvedObject = AlertInvolvedObject{
				Kind:      PodKind,
				Name:      pod,
				Namespace: namespace,
			}
		} else if kind == NodeKind {
			alert.InvolvedObject = AlertInvolvedObject{
				Kind: NodeKind,
				Name: node,
			}
		} else {
			name := instance
			if !instanceOk {
				name = job
			}
			alert.InvolvedObject = AlertInvolvedObject{
				Kind: kind,
				Name: name,
			}
		}
	}

	return alert, nil
}

func DecodeAlertManagerAlerts(r io.ReadCloser) (*AlertManagerAlerts, error) {
	defer r.Close()

	alerts := &AlertManagerAlerts{
		Alerts:            make([]AlertManagerAlert, 0),
		CommonAnnotations: make(map[string]string),
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, alerts); err != nil {
		return nil, err
	}

	return alerts, err
}
