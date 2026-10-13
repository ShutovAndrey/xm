package company

import "errors"

type Type string

type Company struct {
	ID             string
	Name           string
	Description    *string
	EmployeesCount int32
	IsRegistered   bool
	Type           Type
}

var (
	ErrNotFound  = errors.New("company not found")
	ErrNameTaken = errors.New("company name already taken")
	ErrInvalidID = errors.New("invalid company id")
)
