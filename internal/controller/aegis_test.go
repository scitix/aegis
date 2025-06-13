package controller

import (
	"regexp"
	"testing"

	"github.com/scitix/aegis/api/models"
)

func TestGenerateName(t *testing.T) {
	expectedMap := map[*models.Alert]string{
		&models.Alert{
			AlertSourceType: "aaa.bbb",
			Type:            "NodeNotReady",
		}: "aaa-bbb-nodenotready-",
		&models.Alert{
			AlertSourceType: "aaa_bbb",
			Type:            "Node-Crash",
		}: "aaa-bbb-node-crash-",
	}

	for _alert, expected := range expectedMap {
		if got := getGeneratename(_alert); expected != got {
			t.Errorf("Got unexpected name, expected: %s, got: %s", expected, got)
		}
	}
}

func TestRegexpLabelValue(t *testing.T) {
	expectedMap := map[string]bool{
		"xfs":  true,
		"/app": false,
		"_aaa": false,
		"A集群B": false,
	}

	for value, expected := range expectedMap {
		if got, err := regexp.MatchString(ValidLabelValueFormat, value); err == nil && got != expected {
			t.Errorf("Got unexpected for value %s, expected: %v, got: %v", value, expected, got)
		} else if err != nil {
			t.Errorf("Error: %v", err)
		}
	}
}
