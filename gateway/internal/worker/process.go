package worker

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

func terminateProcessGroup(
	ctx context.Context,
	command *exec.Cmd,
	waitDone <-chan error,
	gracePeriod time.Duration,
) error {
	if command == nil || command.Process == nil {
		return nil
	}

	if err := signalProcessGroup(command, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to terminate Java worker: %w", err)
	}

	timer := time.NewTimer(gracePeriod)
	defer timer.Stop()

	select {
	case err := <-waitDone:
		return normalizeProcessExit(err)
	case <-timer.C:
		if err := signalProcessGroup(command, syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to force-kill Java worker: %w", err)
		}
	case <-ctx.Done():
		if err := signalProcessGroup(command, syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to force-kill Java worker: %w", err)
		}
	}

	select {
	case err := <-waitDone:
		return normalizeProcessExit(err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func normalizeProcessExit(err error) error {
	if err == nil {
		return nil
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return nil
		}
	}

	return err
}

func signalProcessGroup(command *exec.Cmd, signal syscall.Signal) error {
	if command == nil || command.Process == nil {
		return nil
	}

	if err := syscall.Kill(-command.Process.Pid, signal); err == nil {
		return nil
	}

	return command.Process.Signal(signal)
}
