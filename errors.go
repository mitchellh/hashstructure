package hashstructure

import (
	"fmt"
)

// ErrNotStringer is returned when there's an error with hash:"string"
type ErrNotStringer struct {
	Field string
}

// Error implements error for ErrNotStringer
func (ens *ErrNotStringer) Error() string {
	return fmt.Sprintf("hashstructure: %s has hash:\"string\" set, but does not implement fmt.Stringer", ens.Field)
}
