package idempotent

import (
	"fmt"

	"github.com/rprtr258/mk/conc"
	"go.uber.org/multierr"
)

// Sentinel result for actions that have nothing to return.
type Sentinel struct{}

// Action represents action that can be retried several times.
// If action was alreadey completed, there is no need to run it again.
type Action[T any] interface {
	// IsCompleted - check whether action need to be run.
	IsCompleted() (bool, error)
	// Perform action and return some result. After success, IsCompleted must return true.
	Perform() (T, error)
}

type sentinelAction[T any] struct {
	action Action[T]
}

func (a sentinelAction[T]) IsCompleted() (bool, error) {
	return a.action.IsCompleted() //nolint:wrapcheck // return original error
}

func (a sentinelAction[T]) Perform() (Sentinel, error) {
	_, err := a.action.Perform()
	return Sentinel{}, err //nolint:wrapcheck // return original error
}

func RemoveResult[T any](action Action[T]) Action[Sentinel] {
	return sentinelAction[T]{action: action}
}

// Perform single action. If it is already completed, no action is performed.
func Perform(action Action[Sentinel]) error {
	completed, errCompleted := action.IsCompleted()
	if errCompleted != nil {
		return fmt.Errorf("check isCompleted: %w", errCompleted)
	}

	if completed {
		return nil
	}

	_, err := action.Perform()
	return err //nolint:wrapcheck // return original error
}

// Multistep - perform idempotent actions by given order.
func Multistep(actions ...Action[Sentinel]) error {
	for i, action := range actions {
		if err := Perform(action); err != nil {
			return fmt.Errorf("action #%d: %w", i, err)
		}
	}

	return nil
}

func Parallel(actions ...Action[Sentinel]) error {
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
			if err := Perform(action); err != nil {
				errch <- err
			}
		})
	}
	wg.Wait()
	close(errch)

	return fmt.Errorf("parallel: %w", multierr.Combine(errs...))
}
