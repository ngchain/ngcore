package utils

import "google.golang.org/protobuf/proto"

type pb struct {
	proto.MarshalOptions
	*proto.UnmarshalOptions
}

var Proto = &pb{
	proto.MarshalOptions{Deterministic: true},
	&proto.UnmarshalOptions{},
}
