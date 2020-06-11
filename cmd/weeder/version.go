package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// nolint
var (
	Version            = "0.0.0"
	SeaweedfsSupported = ""
	GitCommit          string
	GoVersion          string
	BuidDate           string
	ShortDescription   = "Proxy for the Seaweedfs"
)

// cmdVersion command for showing version info
func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "version",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := `Weeder - %s
version:	        %s
revision:	        %s
SeaweedfsSupported:	%s
compile:	        %s
go version:	        %s
`

			fmt.Printf(s, ShortDescription, Version, GitCommit,
				SeaweedfsSupported, BuidDate, GoVersion)
			return nil
		},
	}
}
