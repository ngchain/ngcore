package ngtypes

// some pre-defined extra format for the smart contacts

// AppendExtra is the pre-defined data strcut for the Extra field in Append Tx
type AppendExtra struct {
	Pos     uint64
	Content []byte
}

// DeleteExtra is the pre-defined data struct for the Extra field in Delete Tx
type DeleteExtra struct {
	Pos     uint64
	Content []byte
}
