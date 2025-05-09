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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/log"
	"github.com/lxc/incus/v6/shared/api"
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

	var isTrueNAS bool

	var enableSSH bool
	var addGPU bool
	var profiles []string
	var validBridges []string

	isTrueNAS, err = c.global.client.IsTrueNAS(c.Command().Context())
	if err != nil {
		log.Error("Error checking if TrueNAS:", "error", err)
		return err
	}
	if isTrueNAS {
		profiles, err := c.global.client.ProfileNames(c.Command().Context())
		if err != nil {
			log.Error("Error getting profiles:", "error", err)
			return err
		}
		found := false
		for _, p := range profiles {
			if p == "scriptcli-storage" {
				found = true
				log.Debug("Found TrueNAS profile", "profile", p)
				launchSettings.Profiles = append(launchSettings.Profiles, p)
			}
		}
		if !found {
			log.Info("No TrueNAS profiles found")
			// get the list of pools
			pools, err := c.global.client.StorageList(c.Command().Context())
			if err != nil {
				log.Error("Error getting incus storage pools:", "error", err)
				return err
			}
			if len(pools) == 0 {
				log.Error("No storage pools found")
				return errors.New("no storage pools found")
			}
			// get the default pool
			defaultPool := ""
			for _, pool := range pools {
				if pool.Name == "default" {
					defaultPool = pool.Name
					break
				}
			}
			if defaultPool == "" {
				log.Info("No default storage pool found, defaulting to first pool")
				defaultPool = pools[0].Name
			}
			log.Debug("Using storage pool", "pool", defaultPool)
			// create the profile
			p := api.ProfilesPost{
				Name: "scriptcli-storage",
				ProfilePut: api.ProfilePut{
					Config:      map[string]string{},
					Description: "TrueNAS storage profile for script-cli",
					Devices: map[string]map[string]string{
						"root": {
							"path": "/",
							"pool": defaultPool,
							"type": "disk",
						},
					},
				},
			}
			err = c.global.client.ProfileCreate(c.Command().Context(), p)
			if err != nil {
				log.Error("Error getting profiles:", "error", err)
				return err
			}
			log.Info("Created Incus profile", "profile", p.Name)
			launchSettings.Profiles = append(launchSettings.Profiles, "scriptcli-storage")

		}
	}

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
	launchSettings.Image = "images:" + application.InstallMethods[0].Resources.Image()
	log.Info("Selected image", "image", launchSettings.Image)
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
							huh.NewOption(application.InstallMethods[0].Resources.GetOS(), 0),
							huh.NewOption(application.InstallMethods[1].Resources.GetOS(), 1),
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
	extraConfigs["environment.PCT_OSTYPE"] = application.InstallMethods[launchSettings.InstallMethod].Resources.GetOS()

	// OS Version
	extraConfigs["environment.PCT_OSVERSION"] = application.InstallMethods[launchSettings.InstallMethod].Resources.GetVersion()

	// tz
	extraConfigs["environment.tz"] = "Etc/UTC"

	// Cacher
	extraConfigs["environment.CACHER"] = "no"
	extraConfigs["environment.DEBIAN_FRONTEND"] = "noninteractive"

	// Disable ipv6
	extraConfigs["environment.DISABLEIPV6"] = "yes" // todo: make this a form option

	if disableSecureBoot(application.InstallMethods[launchSettings.InstallMethod].Resources.GetOS()) {
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
	if application.InstallMethods[launchSettings.InstallMethod].Resources.GetOS() == "alpine" {
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
	//extraConfigs["environment.FUNCTIONS_FILE_PATH"] = string(funcScript)

	extraConfigs["environment.FUNCTIONS_FILE_PATH"] = "/install.func"
	log.Info("Preparing image", "image", launchSettings.Image)

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

		dir, err := os.MkdirTemp("", "scriptcli-installfunc")
		if err != nil {
			fmt.Println("Error creating temp dir:", err)
			os.Exit(1)
		}
		defer os.RemoveAll(dir)
		// write the install script to a file
		modifiedScript := []byte("#!/bin/env bash\n")
		modifiedScript = append(modifiedScript, funcScript...)
		modifiedScript = append(modifiedScript, []byte("\n")...)
		err = os.WriteFile(filepath.Join(dir, "install.func"), modifiedScript, 0644)
		if err != nil {
			fmt.Println("Error writing install script:", err)
			os.Exit(1)
		}
		// push the install script to the instance
		log.Info("Adding installation functions to instance...")
		err = exec.Command("incus", "file", "push", filepath.Join(dir, "install.func"), launchSettings.Name+"/install.func").Run()
		if err != nil {
			fmt.Println("Error pushing functions file:", err)
			return err
		}
		// push the install script to the instance
		log.Info("Making installation functions executable...")
		err = exec.Command("incus", "exec", launchSettings.Name, "--", "chmod", "+x", "/install.func").Run()
		if err != nil {
			fmt.Println("Error making functions file executable:", err)
			os.Exit(0)
		}
		log.Info("Running installer...")

		insFunc := string(installFunc)
		insFunc = strings.ReplaceAll(insFunc, "/dev/stdin <<<", "")
		// run installer
		command := exec.Command("incus", "exec", launchSettings.Name, "--", "bash", "-c", string(insFunc))
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		command.Run()

		// err = c.global.client.ExecInteractive([]string{launchSettings.Name, "bash", "-c", string(insFunc)}, []string{}, 0, 0, "", os.Stdin, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Println("Error executing installer:", err)
			os.Exit(1)
		}

		// print the summary
		out, _ := WelcomeMessage(*application, launchSettings)
		output, _ := glamour.Render(out, "dark")
		fmt.Print(output)
		if isTrueNAS {
			log.Info("Removing setup script from instance...")
			err = exec.Command("incus", "config", "set", launchSettings.Name, "environment.FUNCTIONS_FILE_PATH", "").Run()
			if err != nil {
				fmt.Println("Error removing functions file path:", err)
				return err
			}
		}
		log.Info("Removing setup script from instance...")
		err = exec.Command("incus", "config", "set", launchSettings.Name, "environment.DEBIAN_FRONTEND", "").Run()
		if err != nil {
			fmt.Println("Error removing functions file path:", err)
			return err
		}
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
