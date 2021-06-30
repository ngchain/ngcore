package ngtypes

// some pre-defined extra format for the smart contacts

type AppendExtra struct {
	Pos     uint64
	Content []byte
}

type DeleteExtra struct {
	Pos     uint64
	Content []byte
}
