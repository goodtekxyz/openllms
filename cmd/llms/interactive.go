package main

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// Interactive UX (best-practice, short):
//
//   - Esc cancels the current form (prints "cancelled").
//   - Prefer one multi-field form over many confirm steps.
//   - Confirm (Yes/No) only for destructive actions (delete).
//   - No Cancel/Back menu chrome; no Save/Done gates.

var errCancelled = errors.New("cancelled")

func formErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return errCancelled
	}
	return err
}

func exitInteractive(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errCancelled) {
		fmt.Println("cancelled")
		return nil
	}
	return err
}

func runSelect(title string, opts []huh.Option[string], dest *string) error {
	*dest = ""
	return formErr(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title(title).Description("Esc cancels").Options(opts...).Value(dest),
	)).Run())
}

func runSelectFilter(title string, opts []huh.Option[string], dest *string) error {
	*dest = ""
	return formErr(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title(title).Description("Esc cancels").Options(opts...).Value(dest).Filtering(true),
	)).Run())
}

func runConfirm(title string, dest *bool) error {
	return formErr(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title(title).Affirmative("Yes").Negative("No").Value(dest),
	)).Run())
}

func runInput(title, placeholder string, dest *string) error {
	f := huh.NewInput().Title(title).Description("Esc cancels").Value(dest)
	if placeholder != "" {
		f = f.Placeholder(placeholder)
	}
	return formErr(huh.NewForm(huh.NewGroup(f)).Run())
}

// runRouteConfigForm edits preset + account membership in one submit.
func runRouteConfigForm(preset *string, accOpts []huh.Option[string], selected *[]string) error {
	if *preset == "" && len(accOpts) > 0 {
		*preset = "failover"
	}
	return formErr(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Preset").
			Options(routePresetOptions()...).
			Value(preset),
		huh.NewMultiSelect[string]().
			Title("Accounts (order = priority)").
			Description("space toggle · enter submit · Esc cancel").
			Options(accOpts...).
			Value(selected),
	)).Run())
}
