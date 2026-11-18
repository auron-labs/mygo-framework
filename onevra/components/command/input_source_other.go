//go:build !darwin

package command

func selectInputSource(string) (func() error, error) {
	return nil, ErrInputSourceUnavailable
}
