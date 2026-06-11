package domain

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("already exist")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrBadRequest        = errors.New("bad request")
	ErrInternal          = errors.New("internal error")
	ErrInvalid           = errors.New("invalid")
	ErrInsufficientStock = errors.New("insufficient stock")
)
