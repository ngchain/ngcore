package ngstate

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// HandleTxs will apply the tx into the state if tx is VALID
func (s *State) HandleTxs(txs ...*ngtypes.Tx) (err error) {
	err = s.CheckTxs(txs...)
	if err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		switch tx.GetType() {
		case ngtypes.TxType_INVALID:
			return fmt.Errorf("invalid tx")
		case ngtypes.TxType_GENERATE:
			if err := s.handleGenerate(tx); err != nil {
				return err
			}
		case ngtypes.TxType_REGISTER:
			if err := s.handleRegister(tx); err != nil {
				return err
			}
		case ngtypes.TxType_LOGOUT:
			if err := s.handleLogout(tx); err != nil {
				return err
			}
		case ngtypes.TxType_TRANSACTION:
			if err := s.handleTransaction(tx); err != nil {
				return err
			}
		case ngtypes.TxType_ASSIGN: // assign tx
			if err := s.handleAssign(tx); err != nil {
				return err
			}
		case ngtypes.TxType_APPEND: // append tx
			if err := s.handleAppend(tx); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown transaction type")
		}
	}

	return nil
}

func (s *State) handleGenerate(tx *ngtypes.Tx) (err error) {
	rawConvener, exists := s.accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	if err := utils.Proto.Unmarshal(rawConvener, convener); err != nil {
		return err
	}

	publicKey := ngtypes.Address(tx.GetParticipants()[0]).PubKey()
	if err := tx.Verify(publicKey); err != nil {
		return err
	}

	participants := tx.GetParticipants()
	rawParticipantBalance, exists := s.anonymous[base58.FastBase58Encoding(participants[0])]
	if !exists {
		rawParticipantBalance = ngtypes.GetBig0Bytes()
	}

	s.anonymous[base58.FastBase58Encoding(participants[0])] = new(big.Int).Add(
		new(big.Int).SetBytes(rawParticipantBalance),
		new(big.Int).SetBytes(tx.GetValues()[0]),
	).Bytes()

	s.accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) handleRegister(tx *ngtypes.Tx) (err error) {
	log.Debugf("handling new register: %s", tx.BS58())
	rawConvener, exists := s.accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	publicKey := ngtypes.Address(tx.GetParticipants()[0]).PubKey()
	if err = tx.Verify(publicKey); err != nil {
		return err
	}

	totalExpense := new(big.Int).SetBytes(tx.GetFee())

	participants := tx.GetParticipants()
	bs58Addr := base58.FastBase58Encoding(participants[0])
	rawParticipantBalance, exists := s.anonymous[bs58Addr]
	if !exists {
		rawParticipantBalance = ngtypes.GetBig0Bytes()
	}

	if new(big.Int).SetBytes(rawParticipantBalance).Cmp(totalExpense) < 0 {
		return fmt.Errorf("balance is insufficient for register")
	}
	s.anonymous[base58.FastBase58Encoding(participants[0])] = new(big.Int).Sub(
		new(big.Int).SetBytes(rawParticipantBalance),
		totalExpense,
	).Bytes()

	newAccount := ngtypes.NewAccount(binary.LittleEndian.Uint64(tx.GetExtra()), tx.GetParticipants()[0], nil, nil)
	if _, exists := s.accounts[newAccount.Num]; exists {
		return fmt.Errorf("failed to register account@%d", newAccount.Num)
	}

	s.accounts[newAccount.Num], err = utils.Proto.Marshal(newAccount)
	if err != nil {
		return err
	}

	s.accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) handleLogout(tx *ngtypes.Tx) (err error) {
	rawConvener, exists := s.accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()
	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalExpense := new(big.Int).SetBytes(tx.GetFee())

	participants := tx.GetParticipants()
	rawParticipantBalance, exists := s.anonymous[base58.FastBase58Encoding(participants[0])]
	if !exists {
		rawParticipantBalance = ngtypes.GetBig0Bytes()
	}

	if new(big.Int).SetBytes(rawParticipantBalance).Cmp(totalExpense) < 0 {
		return fmt.Errorf("balance is insufficient for logout")
	}
	s.anonymous[base58.FastBase58Encoding(participants[0])] = new(big.Int).Sub(
		new(big.Int).SetBytes(rawParticipantBalance),
		totalExpense,
	).Bytes()

	rawAccount, exists := s.accounts[binary.LittleEndian.Uint64(tx.GetExtra())]
	if !exists {
		return fmt.Errorf("trying to logout an unregistered account")
	}

	delAccount := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawAccount, delAccount)
	if err != nil {
		return err
	}

	if _, exists := s.accounts[delAccount.Num]; !exists {

		return fmt.Errorf("failed to delete account@%d", delAccount.Num)
	}

	delete(s.accounts, delAccount.Num)

	return nil
}

func (s *State) handleTransaction(tx *ngtypes.Tx) (err error) {
	rawConvener, exists := s.accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())
	totalExpense := new(big.Int).Add(fee, totalValue)

	rawConvenerBalance, exists := s.anonymous[base58.FastBase58Encoding(convener.Owner)]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
	if convenerBalance.Cmp(totalExpense) < 0 {
		return fmt.Errorf("balance is insufficient for transaction")
	}

	s.anonymous[base58.FastBase58Encoding(convener.Owner)] = new(big.Int).Sub(convenerBalance, totalExpense).Bytes()

	participants := tx.GetParticipants()
	for i := range participants {
		var rawParticipantBalance []byte
		rawParticipantBalance, exists = s.anonymous[base58.FastBase58Encoding(participants[i])]
		if !exists {
			rawParticipantBalance = ngtypes.GetBig0Bytes()
		}

		s.anonymous[base58.FastBase58Encoding(participants[i])] = new(big.Int).Add(
			new(big.Int).SetBytes(rawParticipantBalance),
			new(big.Int).SetBytes(tx.GetValues()[i]),
		).Bytes()
	}

	s.accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	// DO NOT handle extra
	// TODO: call vm's tx listener

	return nil
}

func (s *State) handleAssign(tx *ngtypes.Tx) (err error) {
	rawConvener, exists := s.accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	rawConvenerBalance, exists := s.anonymous[base58.FastBase58Encoding(convener.Owner)]
	if !exists {
		return fmt.Errorf("account balance does not exist")
	}

	convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
	if convenerBalance.Cmp(fee) < 0 {
		return fmt.Errorf("balance is insufficient for assign")
	}

	s.anonymous[base58.FastBase58Encoding(convener.Owner)] = new(big.Int).Sub(convenerBalance, fee).Bytes()

	// assign the extra bytes
	convener.Contract = tx.GetExtra()

	s.accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) handleAppend(tx *ngtypes.Tx) (err error) {
	rawConvener, exists := s.accounts[tx.GetConvener()]
	if !exists {
		return fmt.Errorf("account does not exist")
	}

	convener := new(ngtypes.Account)
	err = utils.Proto.Unmarshal(rawConvener, convener)
	if err != nil {
		return err
	}

	pk := ngtypes.Address(convener.Owner).PubKey()

	if err = tx.Verify(pk); err != nil {
		return err
	}

	totalValue := ngtypes.GetBig0()
	for i := range tx.GetValues() {
		totalValue.Add(totalValue, new(big.Int).SetBytes(tx.GetValues()[i]))
	}

	fee := new(big.Int).SetBytes(tx.GetFee())

	rawConvenerBalance, exists := s.anonymous[base58.FastBase58Encoding(convener.Owner)]
	if !exists {
		return fmt.Errorf("account balance does not exist")
	}

	convenerBalance := new(big.Int).SetBytes(rawConvenerBalance)
	if convenerBalance.Cmp(fee) < 0 {
		return fmt.Errorf("balance is insufficient for append")
	}

	s.anonymous[base58.FastBase58Encoding(convener.Owner)] = new(big.Int).Sub(convenerBalance, fee).Bytes()

	// append the extra bytes
	convener.Contract = utils.CombineBytes(convener.Contract, tx.GetExtra())
	s.accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	s.accounts[tx.GetConvener()], err = utils.Proto.Marshal(convener)
	if err != nil {
		return err
	}

	return nil
}
