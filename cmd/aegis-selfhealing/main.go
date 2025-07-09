package main

import (
	"os"
	"path/filepath"

	"github.com/scitix/aegis/selfhealing"
	"k8s.io/klog/v2"
)

func main() {
	defer klog.Flush()
	baseName := filepath.Base(os.Args[0])

	if err := selfhealing.NewCommand(baseName).Execute(); err != nil {
		klog.Fatalf("An error occurred: %v", err)
	}
}
