package rpc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/sheet"
	"github.com/ngchain/ngcore/txpool"
	"math/big"
	"net/http"
)

type Tx struct {
	localKey *ecdsa.PrivateKey

	txPool       *txpool.TxPool
	sheetManager *sheet.Manager
}

func NewTxModule(txPool *txpool.TxPool, sheet *sheet.Manager) *Tx {
	return &Tx{
		txPool:       txPool,
		sheetManager: sheet,
	}
}

type SendTxArgs struct {
	Type      int32
	Convener  uint64
	Receivers []uint64
	Values    []uint64
	Fee       uint64
	Nonce     uint64
	Extra     []byte
}

type SendTxReply struct {
	TxHash string
}

func (tx *Tx) SendTx(r *http.Request, args *SendTxArgs, reply *SendTxReply) error {

	convener := tx.txPool.CurrentVault.Sheet.Accounts[args.Convener]
	if convener == nil {
		return fmt.Errorf("convener: %d haven't been registered", args.Convener)
	}

	participants := make([][]byte, len(args.Receivers))
	for i := 0; i < len(args.Receivers); i++ {
		if tx.txPool.CurrentVault.Sheet.Accounts[args.Receivers[i]] == nil {
			return fmt.Errorf("receiver: %d haven't been registered", args.Receivers[i])
		}
		participants[i] = tx.txPool.CurrentVault.Sheet.Accounts[args.Receivers[i]].Owner
	}

	values := make([]*big.Int, len(args.Values))
	for i := 0; i < len(args.Values); i++ {
		values[i] = new(big.Int).SetUint64(args.Values[i])
	}

	newTx := ngtypes.NewUnsignedTransaction(
		args.Type,
		args.Convener,
		participants,
		values,
		new(big.Int).SetUint64(args.Fee),
		args.Nonce,
		args.Extra,
	)

	err := newTx.Signature(tx.localKey)
	if err != nil {
		return err
	}

	err = tx.txPool.PutTxs(newTx)
	if err != nil {
		return err
	}

	reply.TxHash = newTx.HashHex()

	return nil
}

type GetCurrentSheetReply struct {
	Sheet *ngtypes.Sheet
}

func (tx *Tx) GetCurrentSheet(r *http.Request, args *struct{}, reply *GetCurrentSheetReply) error {
	reply.Sheet = tx.sheetManager.GenerateSheet()
	return nil
}

type AccountsReply struct {
	Accounts []*ngtypes.Account
}

func (tx *Tx) ShowLocalAccounts(r *http.Request, args *struct{}, reply *AccountsReply) error {
	key := elliptic.Marshal(elliptic.P256(), tx.localKey.PublicKey.X, tx.localKey.PublicKey.Y)
	reply.Accounts = tx.sheetManager.GetAccountsByPublicKey(key)
	return nil
}
