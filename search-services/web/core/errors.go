package core

import "errors"

var (
	ErrInternal     = errors.New("internal error")
	ErrNotFound     = errors.New("comics not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
)
