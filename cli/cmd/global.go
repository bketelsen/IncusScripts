package main

import (
	"github.com/bketelsen/inclient"
	config "github.com/lxc/incus/v6/shared/cliconfig"
	"github.com/spf13/cobra"
)

type cmdGlobal struct {
	cmd    *cobra.Command
	conf   *config.Config
	client *inclient.Client

	confPath string

	ret int

	flagHelpAll bool
}
