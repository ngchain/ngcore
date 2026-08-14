package ngtypes

import "github.com/pkg/errors"

// Errors for Tx
var (
	// ErrTxSignInvalid occurs when the signature envelope does not verify
	ErrTxSignInvalid    = errors.New("tx signature is invalid")
	ErrTxUnsigned       = errors.New("unsigned tx")
	ErrInvalidPublicKey = errors.New("invalid public key")

	ErrTxNoHeader         = errors.New("tx header is nil")
	ErrTxTypeInvalid      = errors.New("invalid tx type")
	ErrTxRecipientInvalid = errors.New("invalid tx recipient")
	ErrTxValueInvalid     = errors.New("invalid tx value")
	ErrTxFeeInvalid       = errors.New("invalid tx fee")

	ErrTxHeightInvalid = errors.New("invalid tx height")
	ErrTxExtraInvalid  = errors.New("invalid tx extra")
	ErrTxExtraExcess   = errors.New("the size of the tx extra is too large")
)
