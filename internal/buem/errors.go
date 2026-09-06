package buem

import "errors"

// ErrInvalidRequest marks every pre-flight rejection: a request that is
// incomplete or malformed against API contract v5, caught before BuEM is
// called. The HTTP layer maps anything matching it with errors.Is to 400.
// ErrMissingEnvelope and ErrMissingWeather wrap it, as do the geometry and
// start_time checks in task.go and the per-element field checks in
// envelope_validate.go / weather_validate.go.
var ErrInvalidRequest = errors.New("request is incomplete or malformed against API contract v5")
