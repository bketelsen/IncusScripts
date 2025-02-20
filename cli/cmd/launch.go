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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
)

var rootPasswordTwice string
var doit bool

type cmdLaunch struct {
	global *cmdGlobal
}

func (c *cmdLaunch) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "launch <application> <instance name>"
	cmd.Short = "launch a container"
	cmd.Args = cobra.ExactArgs(2)

	cmd.Long =
		`launch a container

Launch a container from the catalog. The application name is the name of the application in the catalog.
The instance name is the name you want to give the container. The instance name must be unique.

Choose "Yes" to use default settings, or "No" to customize the launch settings.

All containers can be launched as a VM. The default is to launch as a container.`
	cmd.RunE = c.Run

	return cmd
}

func (c *cmdLaunch) Run(cmd *cobra.Command, args []string) error {
	app := args[0]
	instanceName := args[1]
	log.Debug("Preparing to launch", "application", app, "instance name", instanceName)
	return c.launch(app, instanceName)

}

func (c *cmdLaunch) launch(app string, instanceName string) error {
	// Should we run in accessible mode?
	accessible, _ := strconv.ParseBool(os.Getenv("ACCESSIBLE"))
	// get the application metadata
	application, err := getAppMetadata(app)
	if err != nil {
		return err
	}
	var advanced bool

	launchSettings := NewLaunchSettings(*application, instanceName)

	if application.Type != "ct" {

		log.Error("Application type not supported", "type", application.Type)
		return errors.New("application type not supported")

	}

	var enableSSH bool
	var addGPU bool
	var profiles []string
	var validBridges []string

	proceed, err := launchForm(app, application.Description, accessible)
	if err != nil {
		return err
	}
	if !proceed {
		log.Error("Instance creation cancelled")
		return nil
	}
	networks, err := c.global.client.Networks(context.Background())
	if err != nil {
		return err
	}
	for _, net := range networks {
		if net.Type == "bridge" {
			validBridges = append(validBridges, net.Name)
		}

	}

	// if it isn't a vm specific application, ask if they want to use the advanced form
	if !launchSettings.VM {
		advanced, err = advancedForm(accessible)
		if err != nil {
			return err
		}
		if advanced {
			// if they want to use the advanced form, ask if they want to run the instance a vm
			launchSettings.VM, err = vmForm(accessible)
			if err != nil {
				return err
			}
		}
	}

	if advanced {

		// select install method
		installMethod := 0
		if len(application.InstallMethods) > 1 {
			// select install method
			form := huh.NewForm(

				huh.NewGroup(
					huh.NewSelect[int]().
						Title("Choose OS Option").
						Options(
							huh.NewOption(application.InstallMethods[0].Resources.OS, 0),
							huh.NewOption(application.InstallMethods[1].Resources.OS, 1),
						).
						Value(&installMethod),
				),
			).WithAccessible(accessible)

			err = form.Run()
			if err != nil {
				fmt.Println("form error:", err)
				os.Exit(1)
			}

		}

		launchSettings.Image = "images:" + application.InstallMethods[installMethod].Resources.Image()
		launchSettings.InstallMethod = installMethod

		if launchSettings.VM {
			// VM Root Disk Size
			// incus launch images:ubuntu/22.04 ubuntu-vm-big --vm --device root,size=30GiB
			defaultDiskSize := fmt.Sprintf("%dGiB", application.InstallMethods[installMethod].Resources.HDD)
			defaultMemory := fmt.Sprintf("%dMiB", application.InstallMethods[installMethod].Resources.RAM)
			launchSettings.VMRootDiskSize = defaultDiskSize
			launchSettings.CPU = application.InstallMethods[installMethod].Resources.CPU
			launchSettings.RAM = defaultMemory
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Value(&launchSettings.VMRootDiskSize).
						Title("Root Disk Size").
						Description("Size of the root disk for the VM.").
						Validate(validateDiskSize),

					huh.NewSelect[int]().
						Value(&launchSettings.CPU).
						Title("Number of CPU Cores").
						Options(huh.NewOptions(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20)...).
						Description("Number of CPU cores to assign the vm."),

					huh.NewInput().
						Value(&launchSettings.RAM).
						Title("VM Memory").
						Placeholder(defaultMemory).
						Description("Memory amount to assign the VM.").
						Validate(validateDiskSize),
				),
			).WithAccessible(accessible)

			err = form.Run()
			if err != nil {
				fmt.Println("form error:", err)
				os.Exit(1)
			}
		}

		// choose ssh options
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Pass through GPU?").
					Value(&addGPU).
					Affirmative("Yes").
					Negative("No"),
			),
		).WithAccessible(accessible)

		err = form.Run()
		if err != nil {
			fmt.Println("form error:", err)
			os.Exit(1)
		}

		// choose ssh options
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Enable SSH?").
					Value(&enableSSH).
					Affirmative("Yes").
					Negative("No"),
			),
		).WithAccessible(accessible)

		err = form.Run()
		if err != nil {
			fmt.Println("form error:", err)
			os.Exit(1)
		}
		launchSettings.EnableSSH = enableSSH

		if enableSSH {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			authKeyFile := ""
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Allow Root SSH with Password?").
						Value(&launchSettings.SSHRootPassword).
						Affirmative("Yes").
						Negative("No"),
				),
				huh.NewGroup(
					huh.NewInput().
						Value(&launchSettings.RootPassword).
						Title("Enter Root Password").
						Placeholder("correct-horse-battery-staple").
						EchoMode(huh.EchoModePassword).
						Description("Root password for the container."),
					huh.NewInput().
						Value(&rootPasswordTwice).
						Title("Confirm Root Password").
						Placeholder("correct-horse-battery-staple").
						EchoMode(huh.EchoModePassword).
						Description("Root password for the container.").
						Validate(func(s string) error {
							if s != launchSettings.RootPassword {
								return errors.New("passwords do not match")
							}
							return nil
						}),
				),
				huh.NewGroup(
					huh.NewFilePicker().
						Value(&authKeyFile).
						Title("SSH Authorized Key").
						FileAllowed(true).
						DirAllowed(false).
						AllowedTypes([]string{".pub"}).
						ShowHidden(true).
						ShowSize(false).
						ShowPermissions(false).
						CurrentDirectory(filepath.Join(home, ".ssh")).
						Description("Press enter to choose a public key file."),
				),
			).WithAccessible(accessible)

			err = form.Run()
			if err != nil {
				fmt.Println("form error:", err)
				os.Exit(1)
			}
			bb, err := os.ReadFile(authKeyFile)
			if err != nil {
				fmt.Println("error reading pub key:", err)

				return err
			}
			launchSettings.SSHAuthorizedKey = string(bb)
		}

		var chooseBridge bool
		// choose advanced network options
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Choose a bridge?").
					Value(&chooseBridge).
					Affirmative("Yes").
					Negative("No"),
			),
		).WithAccessible(accessible)

		err = form.Run()
		if err != nil {
			fmt.Println("form error:", err)
			os.Exit(1)
		}

		if chooseBridge {
			form = huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Value(&launchSettings.Network).
						Title("Choose Network Bridge").
						Options(huh.NewOptions[string](validBridges...)...).
						Description("Select an existing network bridge for the instance."),
				),
			).WithAccessible(accessible)
			err = form.Run()
			if err != nil {
				fmt.Println("form error:", err)
				os.Exit(1)
			}
		}
		// select profiles
		profileList, err := c.global.client.ProfileNames(context.Background())
		if err != nil {
			return err
		}
		// remove "default" profile
		for i, p := range profileList {
			if p == "default" {
				profileList = append(profileList[:i], profileList[i+1:]...)
			}
		}

		// add "default" profile back at the beginning
		profileList = append([]string{"default"}, profileList...)

		var chooseProfiles bool
		// choose advanced network options
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Choose incus profiles?").
					Value(&chooseProfiles).
					Affirmative("Yes").
					Negative("No").
					Description("Select NO to use only the default profile."),
			),
		).WithAccessible(accessible)

		err = form.Run()
		if err != nil {
			fmt.Println("form error:", err)
			os.Exit(1)
		}
		if chooseProfiles {
			form = huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Options(huh.NewOptions(profileList...)...).
						Title("Select Additional Incus Profiles").
						Value(&profiles).
						Description("*default* profile should usually be included."),
				),
			).WithAccessible(accessible)

			err = form.Run()
			if err != nil {
				fmt.Println("form error:", err)
				os.Exit(1)
			}
		}
		launchSettings.Profiles = profiles

	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Create instance?").
				Value(&doit).
				Affirmative("Yes!").
				Negative("No."),
		),
	).WithAccessible(accessible)

	err = form.Run()
	if err != nil {
		fmt.Println("form error:", err)
		os.Exit(1)
	}
	extraConfigs := make(map[string]string)
	deviceOverrides := make(map[string]map[string]string)
	// set environment variables
	// SSH Enable
	if launchSettings.EnableSSH {
		extraConfigs["environment.INSTALL_SSH"] = "yes"
	} else {
		extraConfigs["environment.INSTALL_SSH"] = "no"
	}
	if launchSettings.SSHRootPassword {
		extraConfigs["environment.SSH_ROOT"] = "yes"
	} else {
		extraConfigs["environment.SSH_ROOT"] = "no"
	}
	// SSH Authorized Key
	if len(launchSettings.SSHAuthorizedKey) > 0 {
		extraConfigs["environment.SSH_AUTHORIZED_KEY"] = launchSettings.SSHAuthorizedKey
	} else {
		extraConfigs["environment.SSH_AUTHORIZED_KEY"] = "\"\""
	}
	// Root Password
	if len(launchSettings.RootPassword) > 0 {
		extraConfigs["environment.PASSWORD"] = launchSettings.RootPassword
	} else {
		extraConfigs["environment.PASSWORD"] = "\"\""
	}
	// cttype - container type, always 0
	extraConfigs["environment.CTTYPE"] = "0"
	// app - lower caseed application name
	extraConfigs["environment.app"] = application.Slug

	// Application Name
	extraConfigs["environment.APPLICATION"] = application.Name

	// OS Type
	extraConfigs["environment.PCT_OSTYPE"] = application.InstallMethods[launchSettings.InstallMethod].Resources.OS

	// OS Version
	extraConfigs["environment.PCT_OSVERSION"] = application.InstallMethods[launchSettings.InstallMethod].Resources.Version

	// tz
	extraConfigs["environment.tz"] = "Etc/UTC"

	// Cacher
	extraConfigs["environment.CACHER"] = "no"

	// Disable ipv6
	extraConfigs["environment.DISABLEIPV6"] = "yes" // todo: make this a form option

	if disableSecureBoot(application.InstallMethods[launchSettings.InstallMethod].Resources.OS) {
		launchSettings.VMSecureBoot = false
	}
	if launchSettings.VM {
		deviceOverrides["root"] = map[string]string{"size": launchSettings.VMRootDiskSize}
		extraConfigs["limits.cpu"] = strconv.Itoa(launchSettings.CPU)
		extraConfigs["limits.memory"] = launchSettings.RAM
		if !launchSettings.VMSecureBoot {
			extraConfigs["security.secureboot"] = "false"
		}
	}

	var funcScript []byte
	if application.InstallMethods[launchSettings.InstallMethod].Resources.OS == "alpine" {
		funcScript, err = downloadRaw(repository, "misc", "alpine-install.func")
		if err != nil {
			fmt.Println("download error:", err)
			os.Exit(1)
		}
	} else {
		funcScript, err = downloadRaw(repository, "misc", "install.func")
		if err != nil {
			fmt.Println("download error:", err)
			os.Exit(1)
		}
	}
	// Function script
	extraConfigs["environment.FUNCTIONS_FILE_PATH"] = string(funcScript)

	createInstance := func() {
		// create the instance
		err := c.global.client.Launch(launchSettings.Image, launchSettings.Name, launchSettings.Profiles, extraConfigs, deviceOverrides, launchSettings.Network, launchSettings.VM, false)
		if err != nil {
			fmt.Println("Error creating instance:", err)
			os.Exit(1)
		}
		// TODO add bash to alpine before continuing
		//   if [ "$var_os" == "alpine" ]; then
		//     sleep 3
		//     incus exec "$HN" -- /bin/sh -c 'cat <<EOF >/etc/apk/repositories
		// http://dl-cdn.alpinelinux.org/alpine/latest-stable/main
		// http://dl-cdn.alpinelinux.org/alpine/latest-stable/community
		// EOF'
		//     incus exec "$HN"  -- ash -c "apk add bash >/dev/null"
		//   fi
		if addGPU {
			err = c.global.client.AddDeviceToInstance(context.Background(), launchSettings.Name, "gpu", map[string]string{"type": "gpu", "gid": "44", "uid": "0"})
			if err != nil {
				fmt.Println("Error adding GPU to instance:", err)
				os.Exit(1)
			}
		}
		err = c.global.client.StartInstance(context.Background(), launchSettings.Name)
		if err != nil {
			fmt.Println("Error starting instance:", err)
			os.Exit(1)
		}
		if launchSettings.VM {
			log.Info("VM started, waiting for agent...")
			const maxAttempts = 5
			const waitTime = 2
			getState := func() (bool, error) {
				time.Sleep(waitTime * time.Second)
				state, err := c.global.client.InstanceState(context.Background(), launchSettings.Name)
				if err != nil {
					fmt.Println("Error waiting for vm agent:", err)
					return false, err
				}
				if state.State.Processes > 2 {
					return true, nil
				}
				return false, nil
			}
			attempts := 0
			for {
				success, err := getState()
				if err != nil {
					fmt.Println("Error waiting for vm agent:", err)
					os.Exit(1)
				}
				if success {
					break
				}
				attempts++
				if attempts >= maxAttempts {
					fmt.Println("Error waiting for vm agent: max attempts reached")
					os.Exit(1)
				}
			}
		}
	}

	if doit {
		_ = spinner.New().Title("Creating instance...").Accessible(accessible).Action(createInstance).Run()
		installFunc, err := downloadRaw(repository, "install", application.Slug+"-install.sh")
		if err != nil {
			fmt.Println("Error downloading install script:", err)
			os.Exit(1)
		}
		// run installer
		err = c.global.client.ExecInteractive([]string{launchSettings.Name, "bash", "-c", string(installFunc)}, []string{}, 0, 0, "", os.Stdin, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Println("Error executing installer:", err)
			os.Exit(1)
		}

		// print the summary
		out, _ := WelcomeMessage(*application, launchSettings)
		output, _ := glamour.Render(out, "dark")
		fmt.Print(output)
	} else {
		log.Error("Instance creation cancelled")
	}
	return nil
}

func disableSecureBoot(imagename string) bool {
	return strings.Contains(imagename, "archlinux")

}

func getAppMetadata(app string) (*Application, error) {
	log.Debug("Downloading application metadata", "application", app)
	appJson, err := downloadRaw(repository, "json", app+".json")
	if err != nil {
		log.Error("Failed to download application metadata:", "error", err)
		return nil, err
	}
	var application Application
	err = json.Unmarshal(appJson, &application)
	if err != nil {
		log.Error("Failed to parse application metadata:", "error", err)
		return nil, err
	}
	return &application, nil
}
