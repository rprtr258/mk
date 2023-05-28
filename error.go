package mk

import "fmt"

func Must0(err error) {
	if err != nil {
		panic(err)
	}
}

func Must1[A any](a A, err error) A {
	if err != nil {
		panic(err)
	}

	return a
}

func Must2[A, B any](a A, b B, err error) (A, B) {
	if err != nil {
		panic(err)
	}

	return a, b
}

func handlePanic(p any) error {
	if e, ok := p.(error); ok {
		return e
	}
	return fmt.Errorf("panic: %#v", p)
}

func Try0(f func()) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = handlePanic(p)
		}
	}()

	f()
	return nil
}

func Try1[A any](f func() A) (_ A, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = handlePanic(p)
		}
	}()

	return f(), nil
}

func Try2[A, B any](f func() (A, B)) (_ A, _ B, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = handlePanic(p)
		}
	}()

	a, b := f()
	return a, b, nil
}
