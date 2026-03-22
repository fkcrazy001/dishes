package store

import "errors"

type AppError struct {
	Code    string
	Details any
}

func (e *AppError) Error() string { return e.Code }

func Err(code string, details any) error {
	return &AppError{Code: code, Details: details}
}

func AsAppError(err error) (*AppError, bool) {
	var ae *AppError
	if err == nil {
		return nil, false
	}
	if ok := errors.As(err, &ae); ok {
		return ae, true
	}
	return nil, false
}
