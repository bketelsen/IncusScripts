package main

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

func launchForm(appName string, description string, accessible bool) (bool, error) {
	var proceed bool

	form := huh.NewForm(

		huh.NewGroup(huh.NewNote().
			Title("Incus Scripts").
			Description(fmt.Sprintf("Launch a _%s_ instance\n\n%s\n\n", appName, description)),
			huh.NewConfirm().
				Title("Continue (y/n)?").
				Value(&proceed).
				Affirmative("Yes").
				Negative("No"),
		),
	).WithAccessible(accessible)

	err := form.Run()
	if err != nil {
		fmt.Println("form error:", err)
		return false, err
	}
	return proceed, nil
}
func advancedForm(accessible bool) (bool, error) {
	var advanced bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Use Advanced Settings?").
				Affirmative("Yes").
				Negative("No").
				Value(&advanced),
		),
	).WithAccessible(accessible)

	err := form.Run()
	if err != nil {
		fmt.Println("form error:", err)
		return false, err
	}
	return advanced, nil
}

func vmForm(accessible bool) (bool, error) {
	var vm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Run as Virtual Machine?").
				Affirmative("Yes").
				Negative("No").
				Value(&vm),
		),
	).WithAccessible(accessible)

	err := form.Run()
	if err != nil {
		fmt.Println("form error:", err)
		return false, err
	}
	return vm, nil
}
