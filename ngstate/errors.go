package ngstate

import "github.com/pkg/errors"

var (
	ErrSnapshotNofFound = errors.New("cannot find the snapshot")

	// ErrContractActive occurs when committing to / destroying / activating
	// an already-active contract
	ErrContractActive = errors.New("contract is active")
	// ErrContractNotActive occurs when deactivating an inactive contract
	ErrContractNotActive = errors.New("contract is not active")
)
