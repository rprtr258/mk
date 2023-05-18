package idempotent

import "fmt"

// Action represents action that can be retried several times.
// If action was alreadey completed, there is no need to run it again.
type Action interface {
	// IsCompleted - check whether action need to be run.
	IsCompleted() (bool, error)
	// Perform action. After success, IsCompleted must return true.
	Perform() error
}

func Perform(action Action) error {
	completed, errCompleted := action.IsCompleted()
	if errCompleted != nil {
		return fmt.Errorf("check isCompleted: %w", errCompleted)
	}

	if completed {
		return nil
	}

	return action.Perform() //nolint:wrapcheck // return original error
}

func Multistep(actions ...Action) error {
	for i, action := range actions {
		if err := Perform(action); err != nil {
			return fmt.Errorf("action #%d: %w", i, err)
		}
	}

	return nil
}
