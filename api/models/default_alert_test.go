package models

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"testing"
)

func TestValidate(t *testing.T) {
	validAlerts := map[*Alert]bool{
		&Alert{
			Type: "",
		}: false,
		&Alert{
			Type:   "NodeNotReady",
			Status: "Active",
		}: false,
		&Alert{
			Type:   "NodeNotReady",
			Status: AlertStatusFiring,
			InvolvedObject: AlertInvolvedObject{
				Kind: NodeKind,
				Name: "node1",
			},
		}: true,
		&Alert{
			Type:   "NodeNotReady",
			Status: AlertStatusFiring,
			InvolvedObject: AlertInvolvedObject{
				Kind:      NodeKind,
				Name:      "node1",
				Namespace: "kube-system",
			},
		}: true,
		&Alert{
			Type:   "NodeNotReady",
			Status: AlertStatusFiring,
			InvolvedObject: AlertInvolvedObject{
				Kind:      PodKind,
				Name:      "node1",
				Namespace: "kube-system",
			},
		}: true,
		&Alert{
			Type:   "NodeNotReady",
			Status: AlertStatusFiring,
			InvolvedObject: AlertInvolvedObject{
				Kind: "Cluster",
				Name: "cluster1",
			},
		}: false,
		&Alert{
			Type:   "NodeNotReady",
			Status: AlertStatusFiring,
			InvolvedObject: AlertInvolvedObject{
				Kind: NodeKind,
			},
		}: false,
		&Alert{
			Type:   "NodeNotReady",
			Status: AlertStatusFiring,
			InvolvedObject: AlertInvolvedObject{
				Kind:      PodKind,
				Name:      "node1",
				Namespace: "",
			},
		}: false,
	}

	for alert, valid := range validAlerts {
		if err := alert.Validate(); (err == nil) == valid {
			t.Logf("Got expected result, expected validation: %v, got: %v", valid, err)
		} else {
			t.Errorf("Got error result, expected validation: %v, got: %v", valid, err)
		}
	}
}

func TestDecodeAlert(t *testing.T) {
	alert := &Alert{
		Type:   "NodeNotReady",
		Status: AlertStatusFiring,
		InvolvedObject: AlertInvolvedObject{
			Kind: NodeKind,
			Name: "node1",
		},
		Details: map[string]string{
			"message": "an alert from test",
		},
	}

	b, _ := json.Marshal(alert)
	rw := ioutil.NopCloser(bytes.NewReader(b))
	if a, err := DecodeAlert(rw); err != nil {
		t.Errorf("decode alert error: %v", err)
	} else {
		t.Logf("decode alert: %+v", a)
	}
}
