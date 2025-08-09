package models

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/go-openapi/strfmt"
)

func TestConvertAlertmanagerAlert(t *testing.T) {
	validAlerts := map[*AlertManagerAlert]bool{
		&AlertManagerAlert{}: false,
		&AlertManagerAlert{
			Status: "",
		}: false,
		&AlertManagerAlert{
			Status: "active",
		}: false,
		&AlertManagerAlert{
			Status: "Firing",
		}: false,
		&AlertManagerAlert{
			Status: "Firing",
			Labels: map[string]string{
				"alertname": "NodeNotReady",
				"node":      "node1",
			},
		}: true,
		&AlertManagerAlert{
			Status: "Firing",
			Labels: map[string]string{
				"alertname": "NodeNotReady",
				"instance":  "node1",
			},
		}: true,
		&AlertManagerAlert{
			Status: "Firing",
			Labels: map[string]string{
				"alertname": "NodeNotReady",
				"node":      "node1",
				"instance":  "node1",
			},
		}: true,
		&AlertManagerAlert{
			Status: "Firing",
			Labels: map[string]string{
				"node": "node1",
			},
		}: false,
		&AlertManagerAlert{
			Status: "Firing",
			Labels: map[string]string{
				"alertname": "NodeNotReady",
			},
		}: false,
		&AlertManagerAlert{
			Status: "Resolved",
			Labels: map[string]string{
				"alertname": "PodNotReady",
				"node":      "node1",
				"pod":       "pod-xxx",
				"namespace": "test",
			},
		}: true,
		&AlertManagerAlert{
			Status: "Resolved",
			Labels: map[string]string{
				"alertname": "PodNotReady",
				"instance":  "node1",
				"pod":       "pod-xxx",
				"namespace": "test",
			},
		}: true,
		&AlertManagerAlert{
			Status: "Resolved",
			Labels: map[string]string{
				"alertname": "PodNotReady",
				"pod":       "pod-xxx",
				"namespace": "test",
			},
		}: false,
		&AlertManagerAlert{
			Status: "Resolved",
			Labels: map[string]string{
				"alertname": "PodNotReady",
				"node":      "node1",
				"pod":       "pod-xxx",
			},
		}: false,
		&AlertManagerAlert{
			Status: "Firing",
			Labels: map[string]string{
				"alertname":            "NodeNotReady",
				"involved_object_kind": "node",
				"involved_object_name": "node1",
			},
		}: true,
		&AlertManagerAlert{
			Status: "Firing",
			Labels: map[string]string{
				"alertname":            "PodNotReady",
				"involved_object_kind": "pod",
				"involved_object_name": "pod-xxxx",
				"namespace":            "test",
			},
		}: true,
		&AlertManagerAlert{
			Status: "Firing",
			Labels: map[string]string{
				"alertname":            "PodNotReady",
				"involved_object_kind": "pod",
				"namespace":            "test",
			},
		}: false,
		&AlertManagerAlert{
			Status: "Firing",
			Labels: map[string]string{
				"alertname":            "PodNotReady",
				"involved_object_name": "pod-xxxx",
				"namespace":            "test",
			},
		}: false,
	}

	for alert, valid := range validAlerts {
		if _, err := alert.ConvertAlertmanagerToCommonAlert(); (err == nil) == valid {
			t.Logf("Got expected result, expected validation: %v, got: %v", valid, err)
		} else {
			t.Errorf("Got error result, expected validation: %v, got: %v", valid, err)
		}
	}
}

func TestDecodeAlertManagerAlert(t *testing.T) {
	alerts := &AlertManagerAlerts{
		Status: "Firing",
		Alerts: []AlertManagerAlert{
			{
				StartsAt:  &strfmt.DateTime{},
				EndsAt:    &strfmt.DateTime{},
				UpdatedAt: &strfmt.DateTime{},
				Status:    "Firing",
				Labels: map[string]string{
					"alertname": "NodeNotReady",
					"kind":      "Node",
					"node":      "node1",
					"pod":       "pod-xx1",
				},
			},
			{
				StartsAt:  &strfmt.DateTime{},
				EndsAt:    &strfmt.DateTime{},
				UpdatedAt: &strfmt.DateTime{},
				Status:    "Firing",
				Labels: map[string]string{
					"alertname": "EtcdBackendQuotaLowSpace",
					"kind":      "Etcd",
					"job":       "serviceMonitor/monitoring/etcd/0",
					"instance":  "1.2.3.4:2379",
				},
			},
		},
	}

	b, _ := json.Marshal(alerts)
	rw := ioutil.NopCloser(bytes.NewReader(b))
	if a, err := DecodeAlertManagerAlerts(rw); err != nil {
		t.Errorf("decode alertmanager alert error: %v", err)
	} else {
		t.Logf("decode alertmanager alert: %+v", a)
	}
}

func TestDecodeAlertManagerAlertContent(t *testing.T) {
	content := `"{}"`
	rw := ioutil.NopCloser(bytes.NewReader([]byte(content)))

	if a, err := DecodeAlertManagerAlerts(rw); err != nil {
		t.Errorf("decode alertmanager alert error: %v", err)
	} else {
		t.Logf("decode alertmanager alert: %+v", a)
	}
}
