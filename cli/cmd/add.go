/*
Copyright © 2025 Brian Ketelsen <bketelsen@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

type cmdAdd struct {
	global *cmdGlobal
}

func (c *cmdAdd) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "add <application> <instance name>"
	cmd.Short = "add an application to your instance"
	cmd.Args = cobra.ExactArgs(2)

	cmd.Long =
		`Add an application to your instance

Add an application to your instance.  This command will download the application metadata
and prompt you for any required information to install the application in your running instance.`
	cmd.RunE = c.Run

	return cmd
}

func (c *cmdAdd) Run(cmd *cobra.Command, args []string) error {
	app := args[0]
	instanceName := args[1]
	log.Debug("Preparing to add", "application", app, "instance name", instanceName)
	return c.add(app, instanceName)

}

func (c *cmdAdd) add(app string, instanceName string) error {
	// Should we run in accessible mode?
	accessible, _ := strconv.ParseBool(os.Getenv("ACCESSIBLE"))
	// get the application metadata
	application, err := getAppMetadata(app)
	if err != nil {
		return err
	}

	if application.Type != "misc" {
		log.Error("Application type not supported", "type", application.Type)
		return errors.New("application type not supported")
	}
	form := huh.NewForm(
		huh.NewGroup(huh.NewNote().
			Title("Incus Scripts").
			Description(fmt.Sprintf("Install _%s_ in instance %s\n\n%s\n\n", app, instanceName, application.Description)).
			Next(true).
			NextLabel("Get started"),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Proceed?").
				Affirmative("Yes").
				Negative("No").
				Value(&doit),
		),
	).WithAccessible(accessible)

	err = form.Run()
	if err != nil {
		fmt.Println("form error:", err)
		os.Exit(1)
	}
	if doit {
		installFunc, err := downloadRaw(repository, application.InstallMethods[0].Script)
		if err != nil {
			fmt.Println("Error downloading install script:", err)
			os.Exit(1)
		}
		// run installer
		err = c.global.client.ExecInteractive([]string{instanceName, "bash", "-c", string(installFunc)}, []string{}, 0, 0, "", os.Stdin, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Println("Error executing installer:", err)
			os.Exit(1)
		}

	} else {
		log.Error("Application installation cancelled")
	}
	return nil
}
