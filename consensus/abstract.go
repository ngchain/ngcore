package consensus

// State is an abstract state interface
type State interface {
	CommitOp() (State, error)
}

// Consensus is an abstract consensus interface
type Consensus interface {
	Loop()
	GetCurrentState() (State, error)
	CommitState(State) (State, error)
}

// Op is an abstract operation interface
type Op interface {
}
