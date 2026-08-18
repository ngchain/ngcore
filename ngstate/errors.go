package ngstate

import "github.com/pkg/errors"

var (
	ErrSnapshotNofFound = errors.New("cannot find the snapshot")

	// ErrContractActive occurs when committing to / destroying / activating
	// an already-active contract
	ErrContractActive = errors.New("contract is active")
	// ErrContractNotActive occurs when deactivating an inactive contract
	ErrContractNotActive = errors.New("contract is not active")
	// ErrSourceTooLarge occurs when a commit would grow the contract
	// source past the consensus cap
	ErrSourceTooLarge = errors.New("contract source exceeds the size cap")
)
