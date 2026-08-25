package ngstate

import "github.com/pkg/errors"

var (
	ErrSnapshotNofFound = errors.New("cannot find the snapshot")

	// ErrSourceTooLarge occurs when a deploy would grow the contract
	// source past the consensus cap
	ErrSourceTooLarge = errors.New("contract source exceeds the size cap")

	// ErrImmutable occurs when a re-deploy targets a live contract whose
	// current code exports no `upgrade` hook: such a contract is permanently
	// immutable and its code can never be replaced
	ErrImmutable = errors.New("contract exports no upgrade hook: immutable")

	// commit-reveal private mempool: an effect tx is valid ONLY as the reveal
	// of a prior, in-window, unrevealed commitment

	// ErrTxNotCommitted occurs when an effect tx has no matching unrevealed
	// commitment on chain (or carries an empty Salt)
	ErrTxNotCommitted = errors.New("effect tx is not a valid reveal of a prior commitment")
	// ErrCommitTooRecent occurs when the matched commitment was recorded in
	// the SAME block as the reveal (the anti-same-block-reaction rule); it is
	// subsumed by ErrTxNotCommitted since the in-window lookup is strict h <
	// revealHeight, but kept as a named sentinel for tests/tooling
	ErrCommitTooRecent = errors.New("commitment is in the same block as its reveal")
	// ErrCommitExpired occurs when the matched commitment predates the reveal
	// window (it has been pruned/forfeited)
	ErrCommitExpired = errors.New("commitment is older than the reveal window")
	// ErrCommitUnaffordable occurs when a committer cannot pay its commit fee
	ErrCommitUnaffordable = errors.New("committer cannot afford the commit fee")
	// ErrSaltTooShort occurs when a reveal's salt is below MinSaltSize, which
	// would leave a guessable-content commitment open to a preimage grind
	ErrSaltTooShort = errors.New("reveal salt is shorter than the minimum entropy")
	// ErrNothingToDestroy occurs when an empty-code deploy (a destroy) targets
	// an address that has no live contract slot
	ErrNothingToDestroy = errors.New("empty deploy but no live contract to destroy")
)
