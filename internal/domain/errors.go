package domain

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrAlreadyExists       = errors.New("already exists")
	ErrInvalidInput        = errors.New("invalid input")
	ErrResponsibleAbsent   = errors.New("responsible person not found")
	ErrOverwriteForbidden  = errors.New("overwrite forbidden without force")
	ErrNameNotUnique       = errors.New("person name is not unique")
)
