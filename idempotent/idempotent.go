package idempotent

import (
	"context"
	"fmt"

	"github.com/rprtr258/mk/conc"
	"go.uber.org/multierr"
)

// Action represents action that can be retried several times.
// If action was alreadey completed, there is no need to run it again.
type Action interface {
	// IsCompleted - check whether action need to be run.
	IsCompleted() (bool, error)
	// Run action. After success, IsCompleted must return true.
	Run(context.Context) error
}

// Run single action. If it is already completed, no action is performed.
func Run(ctx context.Context, action Action) error {
	completed, errCompleted := action.IsCompleted()
	if errCompleted != nil {
		return fmt.Errorf("check isCompleted: %w", errCompleted)
	}

	if completed {
		return nil
	}

	return action.Run(ctx) //nolint:wrapcheck // return original error
}

// Multistep - perform idempotent actions by given order.
func Multistep(ctx context.Context, actions ...Action) error {
	for i, action := range actions {
		if err := Run(ctx, action); err != nil {
			return fmt.Errorf("action #%d: %w", i, err)
		}
	}

	return nil
}

func Parallel(ctx context.Context, actions ...Action) error {
	errch := make(chan error)
	errs := []error{}

	wg := &conc.WaitGroup{}
	wg.Go(func() {
		for err := range errch {
			errs = append(errs, err)
		}
	})
	for _, action := range actions {
		action := action
		wg.Go(func() {
			if err := Run(ctx, action); err != nil {
				errch <- err
			}
		})
	}
	wg.Wait()
	close(errch)

	return fmt.Errorf("parallel: %w", multierr.Combine(errs...))
}
