package utils

import (
	"github.com/gogo/protobuf/jsonpb"
	jsoniter "github.com/json-iterator/go"
)

// JSON acts as a global json module
var JSON = jsoniter.ConfigCompatibleWithStandardLibrary

type jsonPB struct {
	*jsonpb.Marshaler
	*jsonpb.Unmarshaler
}

var JSONPB = new(jsonPB)
