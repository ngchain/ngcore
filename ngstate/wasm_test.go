package ngstate_test

// TODO: uncomment when new engine done
// func TestNewWasmVM(t *testing.T) {
//	// requires the consensus here
//	db := storage.InitMemStorage()
//
//	f, _ := os.Open("test/contract.wasm")
//	raw, err := ioutil.ReadAll(f) // TODO: implement a mvp
//	if err != nil {
//		panic(err)
//	}
//
//	_ = db.Update(func(txn *dbolt.Tx) error {
//		contract, err := ngstate.NewVM(txn, ngtypes.NewAccount(500, nil, raw, nil))
//		if err != nil {
//			panic(err)
//		}
//
//		err = contract.InitBuiltInImports()
//		if err != nil {
//			panic(err)
//		}
//
//		err = contract.Instantiate()
//		if err != nil {
//			panic(err)
//		}
//
//		fakeTx := ngtypes.NewUnsignedTx(ngproto.TxType_TRANSACTION,
//			nil,
//			0,
//			[][]byte{ngtypes.GenesisAddress},
//			[]*big.Int{big.NewInt(0)},
//			big.NewInt(0),
//			nil,
//		)
//		contract.CallOnTx(fakeTx) // will receive error but main thread wont panic
//
//		return nil
//	})
//
// }
