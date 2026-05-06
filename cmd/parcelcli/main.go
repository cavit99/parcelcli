package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/cavit99/parcelcli/internal/cli"
)

var version = "dev"

func main() {
	root := cli.NewRoot()
	root.Version = effectiveVersion()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func effectiveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
