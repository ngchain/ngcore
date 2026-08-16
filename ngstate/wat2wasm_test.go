package ngstate

import "github.com/c0mm4nd/wasman/wat"

// mustWat compiles wat authoring text into the wasm binary the chain
// actually stores. wat is a TEST convenience only — the consensus path
// accepts wasm binaries, whatever language produced them. Tests keep
// authoring contracts in wat and deploy the compiled module.
func mustWat(source string) []byte {
	bin, err := wat.Compile([]byte(source))
	if err != nil {
		panic("test wat does not compile: " + err.Error())
	}
	return bin
}
