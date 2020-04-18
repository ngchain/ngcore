// Package rpc is the json-rpc2 module in ngcore
//
// All commands/methods should follow these rules:
// - All (private or public) keys are encoded with base58
// - All bytes are encoded in base64 (forced by protobuf)
// - All numbers are float64, coin uint is NG. So when generating tx, its necessary to multiply the values/fee with 1000000 to make unit be MicroNG
// - All protobuf's bytes are in base64
package rpc
