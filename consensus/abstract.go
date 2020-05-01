package consensus

type State interface {
	CommitOp() (State, error)
}

type Consensus interface {
	GetCurrentState() (State, error)
	CommitState(State) (State, error)
}

type Op interface {
}
