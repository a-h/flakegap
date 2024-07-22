package nixcmd

import "fmt"

func NewCommandError(msg string, err error, output string) error {
	return CommandError{
		Message: msg,
		Err:     err,
		Output:  output,
	}
}

type CommandError struct {
	Message string
	Err     error
	Output  string
}

func (e CommandError) Error() string {
	return fmt.Sprintf("%s: %s", e.Message, e.Err.Error())
}

func (e CommandError) Unwrap() error {
	return e.Err
}

func (e CommandError) Is(target error) bool {
	_, ok := target.(CommandError)
	return ok
}

func (e CommandError) As(target interface{}) bool {
	_, ok := target.(*CommandError)
	return ok
}
