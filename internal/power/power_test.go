package power

import (
	"context"
	"errors"
	"testing"
)

func TestRunCommands(t *testing.T) {
	tests := []struct {
		action Action
		name   string
		args   []string
	}{
		{Shutdown, "systemctl", []string{"poweroff"}},
		{Reboot, "systemctl", []string{"reboot"}},
		{Suspend, "systemctl", []string{"suspend"}},
		{Lock, "loginctl", []string{"lock-session"}},
	}
	for _, tt := range tests {
		var gotName string
		var gotArgs []string
		run := func(_ context.Context, name string, args ...string) error {
			gotName, gotArgs = name, args
			return nil
		}
		if err := Run(context.Background(), tt.action, run); err != nil {
			t.Fatalf("%v: %v", tt.action, err)
		}
		if gotName != tt.name || len(gotArgs) != len(tt.args) || gotArgs[0] != tt.args[0] {
			t.Fatalf("%v ran %s %v, want %s %v", tt.action, gotName, gotArgs, tt.name, tt.args)
		}
	}
}

func TestLockFallsBack(t *testing.T) {
	var calls []string
	run := func(_ context.Context, name string, args ...string) error {
		calls = append(calls, name+" "+args[0])
		if name == "loginctl" {
			return errors.New("not available")
		}
		return nil
	}
	if err := Run(context.Background(), Lock, run); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "loginctl lock-session" || calls[1] != "xdg-screensaver lock" {
		t.Fatalf("calls = %v", calls)
	}
}
