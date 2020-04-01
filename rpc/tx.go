package rpc

import (
	"encoding/hex"
	"math/big"

	"github.com/maoxs2/go-jsonrpc2"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

//import (
//	"crypto/ecdsa"
//	"crypto/elliptic"
//	"fmt"
//	"github.com/ngchain/ngcore/ngsheet"
//	"github.com/ngchain/ngcore/ngtypes"
//	"github.com/ngchain/ngcore/txpool"
//	"math/big"
//	"net/http"
//)
//
//type Tx struct {
//	localKey *ecdsa.PrivateKey
//
//	txPool       *txpool.txPool
//	sheetManager *ngsheet.Manager
//}
//
//func NewTxModule(txPool *txpool.txPool, sheet *ngsheet.Manager) *Tx {
//	return &Tx{
//		txPool:       txPool,
//		sheetManager: sheet,
//	}
//}
//
//type SendTxArgs struct {
//	Type      int32
//	Convener  uint64
//	Receivers []uint64
//	Values    []uint64
//	Fee       uint64
//	Nonce     uint64
//	Extra     []byte
//}
//
//type SendTxReply struct {
//	TxHash string
//}
//
//func (tx *Tx) SendTx(r *http.Request, args *SendTxArgs, reply *SendTxReply) error {
//
//	convener := tx.txPool.CurrentVault.Sheet.Accounts[args.Convener]
//	if convener == nil {
//		return fmt.Errorf("convener: %d haven't been registered", args.Convener)
//	}
//
//	participants := make([][]byte, len(args.Receivers))
//	for i := 0; i < len(args.Receivers); i++ {
//		if tx.txPool.CurrentVault.Sheet.Accounts[args.Receivers[i]] == nil {
//			return fmt.Errorf("receiver: %d haven't been registered", args.Receivers[i])
//		}
//		participants[i] = tx.txPool.CurrentVault.Sheet.Accounts[args.Receivers[i]].Owner
//	}
//
//	values := make([]*big.Int, len(args.Values))
//	for i := 0; i < len(args.Values); i++ {
//		values[i] = new(big.Int).SetUint64(args.Values[i])
//	}
//
//	newTx := ngtypes.NewUnsignedTransaction(
//		args.Type,
//		args.Convener,
//		participants,
//		values,
//		new(big.Int).SetUint64(args.Fee),
//		args.Nonce,
//		args.Extra,
//	)
//
//	err := newTx.Signature(tx.localKey)
//	if err != nil {
//		return err
//	}
//
//	err = tx.txPool.PutTxs(newTx)
//	if err != nil {
//		return err
//	}
//
//	reply.TxHash = newTx.HashHex()
//
//	return nil
//}
//
//type GetCurrentSheetReply struct {
//	Sheet *ngtypes.Sheet
//}
//
//func (tx *Tx) GetCurrentSheet(r *http.Request, args *struct{}, reply *GetCurrentSheetReply) error {
//	reply.Sheet = tx.sheetManager.GenerateSheet()
//	return nil
//}
//
//type AccountsReply struct {
//	Accounts []*ngtypes.Account
//}
//
//func (tx *Tx) ShowLocalAccounts(r *http.Request, args *struct{}, reply *AccountsReply) error {
//	key := elliptic.Marshal(elliptic.P256(), tx.localKey.PublicKey.X, tx.localKey.PublicKey.Y)
//	reply.Accounts = tx.sheetManager.GetAccountsByPublicKey(key)
//	return nil
//}

type sendTxParams struct {
	Convener     uint64
	Participants []string
	Values       []float64
	Fee          float64
	Extra        []byte
}

func (s *Server) sendTxFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params sendTxParams
	err := utils.Json.Unmarshal(msg.Params, &params)
	if err != nil {
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var participants = make([][]byte, len(params.Participants))
	for i := range params.Participants {
		participants[i], err = hex.DecodeString(params.Participants[i])
		if err != nil {
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
	}

	var values = make([]*big.Int, len(params.Values))
	for i := range params.Values {
		values[i] = new(big.Int).SetUint64(uint64(params.Values[i] * ngtypes.FloatNG))
	}

	fee := new(big.Int).SetUint64(uint64(params.Fee * ngtypes.FloatNG))

	nonce := s.sheetManager.GetNextNonce(params.Convener)

	tx := ngtypes.NewUnsignedTransaction(
		1,
		params.Convener,
		participants,
		values,
		fee,
		nonce,
		params.Extra,
	)

	err = tx.Signature(s.consensus.PrivateKey)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	err = s.txPool.PutNewTxFromLocal(tx)
	if err != nil {
		jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	ok, _ := utils.Json.Marshal(true)
	return jsonrpc2.NewJsonRpcSuccess(msg.ID, ok)
}
