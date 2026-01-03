package store

import "errors"

var ErrInvalidStateTransition = errors.New("Invalid job state transition")
