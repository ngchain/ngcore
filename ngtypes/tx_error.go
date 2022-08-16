package ngtypes

import "github.com/pkg/errors"

// Errors for Tx
var (
	// ErrTxSignInvalid occurs when the signature of the Tx doesnt match the Tx 's caller/account
	ErrTxSignInvalid    = errors.New("signer of tx is not the own of the account")
	ErrTxUnsigned       = errors.New("unsigned tx")
	ErrInvalidPublicKey = errors.New("invalid public key")

	ErrTxNoHeader            = errors.New("tx header is nil")
	ErrTxTypeInvalid         = errors.New("invalid tx type")
	ErrTxConvenerInvalid     = errors.New("invalid tx convener")
	ErrTxParticipantsInvalid = errors.New("invalid tx participants")
	ErrTxValuesInvalid       = errors.New("invalid tx values")
	ErrTxFeeInvalid          = errors.New("invalid tx fee")

	ErrTxHeightInvalid = errors.New("invalid tx height")
	ErrTxExtraInvalid  = errors.New("invalid tx extra")
	ErrTxExtraExcess   = errors.New("the size of the tx extra is too large")
)
