package domain

import "errors"

// ErrUnknownFigura is returned when a figura is not found in the roster.
var ErrUnknownFigura = errors.New("unknown figura")
