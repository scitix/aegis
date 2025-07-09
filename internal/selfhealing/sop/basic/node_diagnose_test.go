package basic

import "testing"

func TestGetDiagnose(t *testing.T) {
	logs := `hint: 通过 ipmi 工具获取 Memory 传感器异常状态
cmd: ipmimonitoring -Q --ignore-unrecognized-events --comma-separated-output --no-header-output --sdr-cache-recreate --output-event-bitmask --output-sensor-state | grep Memory | grep -v Nominal
result: 96,CPU1_C1D1,Memory,Warning,N/A,N/A,0060h
hint: 通过 ipmi 工具获取 Memory 传感器异常状态
dsaddddddddddd
cmd: ipmimonitoring -Q --ignore-unrecognized-events --comma-separated-output --no-header-output --sdr-cache-recreate --output-event-bitmask --output-sensor-state | grep Memory | grep -v Nominal
result: 96,CPU1_C1D1,Memory,Warning,N/A,N/A,0060h
dsada
dsadddddddd


dsaddddd
hint: 通过 ipmi 工具获取 Memory 传感器异常状态
cmd: ipmimonitoring -Q --ignore-unrecognized-events --comma-separated-output --no-header-output --sdr-cache-recreate --output-event-bitmask --output-sensor-state | grep Memory | grep -v Nominal
xadsadsadsad`
	t.Logf("%+v", getDiagnose(logs))
}
