package utils

import "google.golang.org/protobuf/proto"

type pb struct {
	proto.MarshalOptions
	*proto.UnmarshalOptions
}

// Proto is a global deterministic protobuf marshaller and unmarshaller
var Proto = &pb{
	proto.MarshalOptions{Deterministic: true},
	&proto.UnmarshalOptions{},
}
