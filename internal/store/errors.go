package store

import "errors"

var ErrInvalidStateTransition = errors.New("invalid job state transition")
