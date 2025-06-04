package v1alpha1

import "time"

type Rule struct {
	Module string `yaml:"module"`
	// Type    string `yaml:"type"`
	Item    string        `yaml:"item"`
	Reason  string        `yaml:"reason"`
	File    string        `yaml:"file"`
	Level   string        `yaml:"level"`
	Timeout time.Duration `yaml:"timeout"`
}

type Rules struct {
	Rules []Rule `yaml:"rules"`
}

type Results struct {
	Results ResultInfos `json:"results"`
}
