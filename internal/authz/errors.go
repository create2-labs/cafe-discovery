package authz

import "errors"

// ErrPrincipalUserIDRequired is returned by Principal.Validate when the
// propagated X-User-Id is empty.
var ErrPrincipalUserIDRequired = errors.New("authz: principal user_id is required")
