package main

import (
	"flag"

	klog "k8s.io/klog/v2"
)

// quietKlog limits klog noise from k8s client-go for operations that need a quiet terminal output.
// Call this from commands like `tool exec` just before starting SPDY streams.
func quietKlog() {
	klog.InitFlags(nil)
	_ = flag.Set("stderrthreshold", "FATAL")
	_ = flag.Set("v", "0")
	_ = flag.Set("logtostderr", "false")
	_ = flag.Set("alsologtostderr", "false")
}
