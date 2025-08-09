package network

import (
	"strings"
	"testing"
)

func TestCount(t *testing.T) {
	str := "dsadsad\ndsadsa\n"

	t.Logf("count: %v", strings.Count(str, "\n"))
}
