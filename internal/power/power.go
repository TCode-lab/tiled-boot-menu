// Package power maps launcher actions to system commands.
package power

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Action is a session power operation.
type Action int

const (
	Shutdown Action = iota
	Reboot
	Lock
	Suspend
)

func (a Action) String() string {
	switch a {
	case Shutdown:
		return "shutdown"
	case Reboot:
		return "reboot"
	case Lock:
		return "lock"
	case Suspend:
		return "suspend"
	}
	return "unknown"
}

// Runner executes one command. Tests substitute a fake runner.
type Runner func(ctx context.Context, name string, args ...string) error

// Run performs the action, trying fallbacks for screen locking.
func Run(ctx context.Context, action Action, run Runner) error {
	if run == nil {
		run = SystemRunner
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	switch action {
	case Shutdown:
		return run(ctx, "systemctl", "poweroff")
	case Reboot:
		return run(ctx, "systemctl", "reboot")
	case Suspend:
		return run(ctx, "systemctl", "suspend")
	case Lock:
		candidates := [][2]string{
			{"loginctl", "lock-session"},
			{"xdg-screensaver", "lock"},
			{"gnome-screensaver-command", "-l"},
		}
		var last error
		for _, c := range candidates {
			if err := run(ctx, c[0], c[1]); err == nil {
				return nil
			} else {
				last = err
			}
		}
		return fmt.Errorf("lock screen: %w", last)
	}
	return fmt.Errorf("unknown power action %d", action)
}

// SystemRunner starts a command without a shell and waits for it.
func SystemRunner(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}
