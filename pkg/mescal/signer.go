package mescal

import "errors"

// ErrUnavailable indicates that the host cannot create Apple's SAP action
// signatures. Apple currently provides the required signing service through a
// private macOS framework.
var ErrUnavailable = errors.New("the Apple SAP signing service is unavailable")
