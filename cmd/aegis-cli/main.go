package main

import (
	"os"
	"path/filepath"

	"gitlab.scitix-inner.ai/k8s/aegis/cli"
	"k8s.io/klog/v2"
)

func main() {
	defer klog.Flush()
	baseName := filepath.Base(os.Args[0])

	if err := cli.NewCommand(baseName).Execute(); err != nil {
		klog.Fatalf("An error occurred: %v", err)
	}
}
