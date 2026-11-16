package metadata

import (
	"errors"
)

var (
	ErrNotFound             = errors.New("jav: resource not found")
	ErrUnsupportedProvider  = errors.New("jav: unsupported provider")
	ErrUnsupportedOperation = errors.New("jav: unsupported operation")
	// ErrAuthRequired marks endpoints that refuse anonymous requests and need a
	// signed-in account. It is reported separately from "no such resource" so the
	// UI can ask for credentials instead of claiming the data does not exist.
	ErrAuthRequired = errors.New("jav: account login required")
)
