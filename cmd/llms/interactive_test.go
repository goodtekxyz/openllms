package main

import (
	"errors"
	"testing"

	"github.com/charmbracelet/huh"
)

func TestFormErrAborted(t *testing.T) {
	if err := formErr(huh.ErrUserAborted); !errors.Is(err, errCancelled) {
		t.Fatalf("got %v", err)
	}
	if err := formErr(nil); err != nil {
		t.Fatal(err)
	}
}

func TestExitInteractiveCancel(t *testing.T) {
	if err := exitInteractive(errCancelled); err != nil {
		t.Fatalf("want nil got %v", err)
	}
	if err := exitInteractive(errors.New("boom")); err == nil || err.Error() != "boom" {
		t.Fatalf("got %v", err)
	}
}
