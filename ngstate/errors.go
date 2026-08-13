package ngstate

import "github.com/pkg/errors"

var (
	ErrSnapshotNofFound = errors.New("cannot find the snapshot")

	// ErrAccountLocked occurs when appending/deleting/locking a locked account
	ErrAccountLocked = errors.New("account is locked")
	// ErrAccountNotLocked occurs when unlocking an account which is not locked
	ErrAccountNotLocked = errors.New("account is not locked")
)
