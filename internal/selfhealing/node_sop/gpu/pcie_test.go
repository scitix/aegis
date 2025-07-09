package gpu

import (
	"strings"
	"testing"
)

func TestSplitIds(t *testing.T) {
	str := "0000:1d:e"
	t.Logf("%s", strings.SplitN(str, ":", 2)[1])
}
