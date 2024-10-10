package httpapi

import "errors"

// errBoom is a generic downstream failure used across tests to exercise
// the 5xx/502 paths without caring about the specific error text.
var errBoom = errors.New("boom")
