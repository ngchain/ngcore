package ngstate

import "github.com/pkg/errors"

var (
	ErrSnapshotNofFound = errors.New("cannot find the snapshot")

	// ErrAccountActive occurs when committing to / destroying / activating
	// an already-active account
	ErrAccountActive = errors.New("account is active")
	// ErrAccountNotActive occurs when deactivating an inactive account
	ErrAccountNotActive = errors.New("account is not active")
)
