package jsonrpc

import (
	"encoding/binary"
	"encoding/hex"
	"math/big"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// getSheetReply carries a full-state export: the whole sheet (balances,
// contracts with code+context, key registry) as hex RLP, plus the head
// it was taken at. `ngcore fork --rpc` rebuilds an identical local
// chain from this ONE response — rpc-based forking, anvil style.
type getSheetReply struct {
	Network   uint8  `json:"network"`
	Height    uint64 `json:"height"`
	BlockHash string `json:"blockHash"`
	Timestamp uint64 `json:"timestamp"` // the head block's time
	Sheet     string `json:"sheet"`     // hex(rlp(ngtypes.Sheet))
}

// getHeadReply is the light fork-boot info: what `ngcore fork --rpc`
// (lazy mode) needs before fetching any address
type getHeadReply struct {
	Network   uint8  `json:"network"`
	Height    uint64 `json:"height"`
	BlockHash string `json:"blockHash"`
	Timestamp uint64 `json:"timestamp"`
}

func (s *Server) getHeadFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	latest := s.pow.Chain.GetLatestBlock()

	return reply(msg, getHeadReply{
		Network:   uint8(s.pow.State.Network),
		Height:    latest.GetHeight(),
		BlockHash: hex.EncodeToString(latest.GetHash()),
		Timestamp: latest.GetTimestamp(),
	})
}

// getAddressStateReply is one address's full state — the unit of LAZY
// rpc forking: `ngcore fork --rpc` pulls addresses on first touch instead
// of the whole sheet, so forking scales with what the debug session
// actually reads, not with chain size
type getAddressStateReply struct {
	Exists   bool   `json:"exists"`
	Balance  string `json:"balance"`            // decimal raw units
	Contract string `json:"contract,omitempty"` // hex(rlp(ngtypes.Contract)); "" when no slot
}

func (s *Server) getAddressStateFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params addressParams
	if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	addr, err := ngtypes.NewAddressFromBS58(params.Address)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	// historical read (archive) when a height is given, else the tip
	balance := big.NewInt(0)
	var account *ngtypes.Contract
	if params.Height != nil {
		if bal, err := s.pow.State.GetBalanceByAddressAt(addr, *params.Height); err == nil {
			balance = bal
		} else {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		account, _ = s.pow.State.GetContractAt(addr, *params.Height)
	} else {
		if bal, err := s.pow.State.GetTotalBalanceByAddress(addr); err == nil {
			balance = bal
		}
		account, _ = s.pow.State.GetContract(addr)
	}

	res := getAddressStateReply{Balance: balance.String()}
	if account != nil {
		rawAcc, err := rlp.EncodeToBytes(account)
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		res.Contract = hex.EncodeToString(rawAcc)
	}
	res.Exists = balance.Sign() > 0 || res.Contract != ""

	return reply(msg, res)
}

type getSheetParams struct {
	// Height, when set, reconstructs the whole state as of that past block
	// in an isolated scratch db (works on any node with the blocks; may be
	// slow far from the tip). nil dumps the current state
	Height *uint64 `json:"height,omitempty"`
}

func (s *Server) getSheetFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getSheetParams
	if msg.Params != nil {
		if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
	}

	var (
		sheet     *ngtypes.Sheet
		timestamp uint64
	)
	if params.Height != nil {
		block, err := s.pow.Chain.GetBlockByHeight(*params.Height)
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		timestamp = block.GetTimestamp()
		err = s.pow.State.ReconstructAt(*params.Height, func(txn *bbolt.Tx) error {
			var e error
			sheet, e = ngstate.DumpSheetAt(s.pow.State.Network, txn, *params.Height, block.GetHash())
			return e
		})
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
	} else {
		err := s.pow.State.View(func(txn *bbolt.Tx) error {
			var e error
			sheet, e = ngstate.DumpSheetTxn(s.pow.State.Network, txn)
			return e
		})
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		timestamp = s.pow.Chain.GetLatestBlock().GetTimestamp()
	}

	rawSheet, err := rlp.EncodeToBytes(sheet)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, getSheetReply{
		Network:   uint8(sheet.Network),
		Height:    sheet.Height,
		BlockHash: hex.EncodeToString(sheet.BlockHash),
		Timestamp: timestamp,
		Sheet:     hex.EncodeToString(rawSheet),
	})
}

type addressParams struct {
	Address string `json:"address"`
	// Height, when set, asks for the state as of that past block height
	// (archive nodes only); nil reads the current tip
	Height *uint64 `json:"height,omitempty"`
}

func (s *Server) getContractInfoFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params addressParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	addr, err := ngtypes.NewAddressFromBS58(params.Address)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	var account *ngtypes.Contract
	if params.Height != nil {
		account, err = s.pow.State.GetContractAt(addr, *params.Height)
	} else {
		account, err = s.pow.State.GetContract(addr)
	}
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, account)
}

type getContractStorageParams struct {
	Address string  `json:"address"`
	Key     string  `json:"key"`              // hex of the raw storage key
	Height  *uint64 `json:"height,omitempty"` // past height (archive nodes); nil = tip
}

// getContractStorageReply is one storage slot's value with convenience
// decodes for the two standard widths a contract stores
type getContractStorageReply struct {
	Value string  `json:"value"` // lowercase hex, "" when the key is unset
	Len   int     `json:"len"`
	U64   *uint64 `json:"u64,omitempty"`  // set when len in (1..8]
	U256  string  `json:"u256,omitempty"` // decimal, set when len == 32
}

// getContractStorageFunc reads ONE value from a contract's on-chain kv by
// its raw key (hex) — the targeted read external tools (indexers,
// wallets, explorers) need instead of pulling the whole context via
// getContractInfo. Reserved ("_"-prefixed) keys read as unset, mirroring
// the kv host module.
func (s *Server) getContractStorageFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params getContractStorageParams
	if err := utils.JSON.Unmarshal(*msg.Params, &params); err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	addr, err := ngtypes.NewAddressFromBS58(params.Address)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}
	key, err := hex.DecodeString(params.Key)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var account *ngtypes.Contract
	if params.Height != nil {
		account, err = s.pow.State.GetContractAt(addr, *params.Height)
	} else {
		account, err = s.pow.State.GetContract(addr)
	}
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	var val []byte
	// reserved keys read as unset; guard a nil Context defensively (a
	// well-formed slot always has one, but a historical decode shouldn't panic)
	if account.Context != nil && (len(key) == 0 || key[0] != '_') {
		val = account.Context.Get(string(key))
	}

	out := getContractStorageReply{Value: hex.EncodeToString(val), Len: len(val)}
	if n := len(val); n > 0 && n <= 8 {
		var buf [8]byte
		copy(buf[:], val)
		v := binary.LittleEndian.Uint64(buf[:])
		out.U64 = &v
	}
	if len(val) == 32 {
		be := make([]byte, 32)
		for i, b := range val {
			be[31-i] = b
		}
		out.U256 = new(big.Int).SetBytes(be).String()
	}

	return reply(msg, out)
}

type balanceReply struct {
	TotalBalance  string
	MatureBalance string
	LockedBalance string
}

func (s *Server) getBalanceByAddressFunc(msg *jsonrpc2.JsonRpcMessage) *jsonrpc2.JsonRpcMessage {
	var params addressParams

	err := utils.JSON.Unmarshal(*msg.Params, &params)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	addr, err := ngtypes.NewAddressFromBS58(params.Address)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	// historical balance: mature/locked are tip-only concepts, so a
	// height query reports just the total as of that height
	if params.Height != nil {
		bal, err := s.pow.State.GetBalanceByAddressAt(addr, *params.Height)
		if err != nil {
			log.Error(err)
			return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
		}
		return reply(msg, balanceReply{TotalBalance: bal.String()})
	}

	totalBalance, err := s.pow.State.GetTotalBalanceByAddress(addr)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	matureBalance, err := s.pow.State.GetMatureBalanceByAddress(addr)
	if err != nil {
		log.Error(err)
		return jsonrpc2.NewJsonRpcError(msg.ID, jsonrpc2.NewError(0, err))
	}

	return reply(msg, balanceReply{
		TotalBalance:  totalBalance.String(),
		MatureBalance: matureBalance.String(),
		LockedBalance: new(big.Int).Sub(totalBalance, matureBalance).String(),
	})
}
