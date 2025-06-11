package nodecheck

import (
	"strings"
	"testing"

	nodecheckv1alpha1 "github.com/scitix/aegis/pkg/apis/nodecheck/v1alpha1"
	"gopkg.in/yaml.v3"
)

func TestUnmarshalResult(t *testing.T) {
	text := `W0705 15:50:16.006233       1 client_config.go:615] Neither --kubeconfig nor --master was specified.  Using the inClusterConfig.  This might not work.
I0705 15:50:16.091684       1 nodecheck.go:104] start exec rule: &{ib check_ib_mount IBDeviceMountFailed ib_pod_check.sh emergency 10s}
I0705 15:50:16.091724       1 nodecheck.go:104] start exec rule: &{nic check_nic_count NICCountUNexpected check_nic.sh warning 10s}
results:
  ib:
  - item: check_ib_mount
    condition: IBDeviceMountFailed
    level: emergency
    status: false
    message: ""
  nic:
  - item: check_nic_count
    condition: NICCountUNexpected
    level: warning
    status: true
    message: nic count 0 not equal to 2`

	lines := strings.Split(text, "\n")
	var yamlPart string
	for i, line := range lines {
		if strings.HasPrefix(line, "results:") {
			yamlPart = strings.Join(lines[i:], "\n")
		}
	}

	t.Logf("%s", yamlPart)

	result := nodecheckv1alpha1.Results{
		Results: make(map[string]nodecheckv1alpha1.ResourceInfos),
	}

	err := yaml.Unmarshal([]byte(yamlPart), &result)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%+v", result)
}
