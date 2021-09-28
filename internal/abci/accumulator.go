package abci

import (
	"bytes"
	_ "crypto/sha256"
	"fmt"
	"time"

	_ "github.com/AccumulateNetwork/accumulated/smt/pmt"
	"github.com/AccumulateNetwork/accumulated/types/api/transactions"
	"github.com/AccumulateNetwork/accumulated/types/proto"
	statetypes "github.com/AccumulateNetwork/accumulated/types/state"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/version"
	dbm "github.com/tendermint/tm-db"
)

// Accumulator is an ABCI application that accumulates validated transactions in
// a hash tree.
type Accumulator struct {
	abci.BaseApplication

	retainBlocks int64
	chainId      [32]byte
	state        state
	sdb          *statetypes.StateDB
	address      crypto.Address
	txct         int64
	timer        time.Time
	chain        Chain
}

// NewAccumulator returns a new Accumulator.
func NewAccumulator(db dbm.DB, sdb *statetypes.StateDB, privval *privval.FilePV, chain Chain) (*Accumulator, error) {
	state, err := loadState(db)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %v", err)
	}

	app := &Accumulator{
		state: state,
		sdb:   sdb,
		chain: chain,

		//only retain current block, we will manage our own states
		retainBlocks: 1,
	}

	address := privval.Key.PubKey.Address()
	app.address = make([]byte, len(address))
	copy(app.address, address)

	return app, nil
}

var _ abci.Application = (*Accumulator)(nil)

// Info implements github.com/tendermint/tendermint/abci/types.Application.
func (app *Accumulator) Info(req abci.RequestInfo) abci.ResponseInfo {
	//todo: load up the merkle databases to the same state we're at...  We will need to rewind.

	if app.chain == nil {
		panic("Chain Validator Node not set!")
	}

	return abci.ResponseInfo{
		Data:             fmt.Sprintf("{\"size\":%v}", app.state.Size),
		Version:          version.ABCIVersion,
		AppVersion:       Version,
		LastBlockHeight:  app.sdb.BlockIndex(),
		LastBlockAppHash: app.sdb.EnsureRootHash(),
	}
}

// Query implements github.com/tendermint/tendermint/abci/types.Application.
//
// Exposed as Tendermint RCP /abci_query.
func (app *Accumulator) Query(reqQuery abci.RequestQuery) (resQuery abci.ResponseQuery) {
	resQuery.Key = reqQuery.Data
	query := new(proto.Query)
	err := protobuf.Unmarshal(reqQuery.Data, query)
	if err != nil {
		resQuery.Info = "requst is not an Accumulate Query\n"
		resQuery.Code = code.CodeTypeUnauthorized
		return resQuery
	}

	ret, err := app.chain.Query(query)
	if err != nil {
		resQuery.Info = fmt.Sprintf("%v", err)
		resQuery.Code = code.CodeTypeUnauthorized
		return resQuery
	}

	//if we get here, we have a valid state object, so let's return it.
	resQuery.Code = code.CodeTypeOK
	//return a generic state object for the chain and let the query deal with decoding it
	resQuery.Value = ret

	///implement lazy sync calls. If a node falls behind it needs to have several query calls
	///1 get current height
	///2 get block data for height X
	///3 get block data for given hash
	return
}

// InitChain implements github.com/tendermint/tendermint/abci/types.Application.
//
// Called when a chain is created.
func (app *Accumulator) InitChain(req abci.RequestInitChain) abci.ResponseInitChain {
	var networkid [32]byte
	networkid[31] = 1
	app.chainId = networkid

	//register a list of the validators.
	for _, v := range req.Validators {
		app.updateValidator(v)
	}

	return abci.ResponseInitChain{AppHash: app.chainId[:]}
}

// BeginBlock implements github.com/tendermint/tendermint/abci/types.Application.
func (app *Accumulator) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	//Identify the leader for this block, if we are the proposer... then we are the leader.
	app.chain.BeginBlock(BeginBlockRequest{
		IsLeader: bytes.Equal(app.address.Bytes(), req.Header.GetProposerAddress()),
		Height:   req.Header.Height,
		Time:     req.Header.Time,
	})

	app.timer = time.Now()

	// fmt.Printf("Begin Block %d on network id %s\n", req.Header.Height, req.Header.ChainID)

	app.txct = 0

	/*
		app.ValUpdates = make([]types.ValidatorUpdate, 0)

		// Punish validators who committed equivocation.
		for _, ev := range req.ByzantineValidators {
			if ev.Type == types.EvidenceType_DUPLICATE_VOTE {
				addr := string(ev.Validator.Address)
				if pubKey, ok := app.valAddrToPubKeyMap[addr]; ok {
					app.updateValidator(types.ValidatorUpdate{
						PubKey: pubKey,
						Power:  ev.Validator.Power - 1,
					})
					app.logger.Info("Decreased val power by 1 because of the equivocation",
						"val", addr)
				} else {
					app.logger.Error("Wanted to punish val, but can't find it",
						"val", addr)
				}
			}
		}

	*/

	return abci.ResponseBeginBlock{}
}

// CheckTx implements github.com/tendermint/tendermint/abci/types.Application.
//
// Verifies the transaction is sane.
func (app *Accumulator) CheckTx(req abci.RequestCheckTx) (rct abci.ResponseCheckTx) {
	//create a default response
	ret := abci.ResponseCheckTx{Code: 0, GasWanted: 1}

	//the submission is the format of the Tx input
	sub := new(transactions.GenTransaction)

	//unpack the request
	rem, err := sub.UnMarshal(req.Tx)

	//check to see if there was an error decoding the submission
	if len(rem) != 0 || err != nil {
		//reject it
		return abci.ResponseCheckTx{Code: code.CodeTypeEncodingError, GasWanted: 0,
			Log: "Unable to decode transaction"}
	}

	err = app.chain.CheckTx(sub)

	if err != nil {
		ret.Code = 2
		ret.GasWanted = 0
		ret.GasUsed = 0
		ret.Info = fmt.Sprintf("entry check failed %v for url %x, %v \n", sub.TransactionType(), sub.ChainID, err)
		return ret
	}

	//if we get here, the TX, passed reasonable check, so allow for dispatching to everyone else
	return ret
}

// DeliverTx implements github.com/tendermint/tendermint/abci/types.Application.
//
// Verifies the transaction is valid.
func (app *Accumulator) DeliverTx(req abci.RequestDeliverTx) (rdt abci.ResponseDeliverTx) {
	ret := abci.ResponseDeliverTx{GasWanted: 1, GasUsed: 0, Data: []byte(""), Code: code.CodeTypeOK}

	sub := &transactions.GenTransaction{}

	//unpack the request
	//how do i detect errors?  This causes segfaults if not tightly checked.
	_, err := sub.UnMarshal(req.Tx)
	if err != nil {
		return abci.ResponseDeliverTx{Code: code.CodeTypeEncodingError, GasWanted: 0,
			Log: "Unable to decode transaction"}
	}

	//run through the validation node
	err = app.chain.DeliverTx(sub)

	if err != nil {
		ret.Code = code.CodeTypeUnauthorized
		//ret.GasWanted = 0
		//ret.GasUsed = 0
		//we don't care about failure as far as tendermint is concerned.
		ret.Info = fmt.Sprintf("entry check failed %v on validator %v \n", sub.TransactionType(), err)
		return ret
	}

	//now we need to store the data returned by the validator and feed into accumulator
	app.txct++

	return ret
}

// EndBlock implements github.com/tendermint/tendermint/abci/types.Application.
func (app *Accumulator) EndBlock(req abci.RequestEndBlock) (resp abci.ResponseEndBlock) {
	// Select our leader who will initiate consensus on dbvc chain.
	//resp.ConsensusParamUpdates
	//for _, ev := range req.ByzantineValidators {
	//	if ev.Type == types.EvidenceType_DUPLICATE_VOTE {
	//		addr := string(ev.Validator.Address)
	//		if pubKey, ok := app.valAddrToPubKeyMap[addr]; ok {
	//			app.updateValidator(types.ValidatorUpdate{
	//				PubKey: pubKey,
	//				Power:  ev.Validator.Power - 1,
	//			})
	//			app.logger.Info("Decreased val power by 1 because of the equivocation",
	//				"val", addr)
	//		} else {
	//			app.logger.Error("Wanted to punish val, but can't find it",
	//				"val", addr)
	//		}
	//	}
	//}

	return abci.ResponseEndBlock{} //ValidatorUpdates: app.ValUpdates}
}

// Commit implements github.com/tendermint/tendermint/abci/types.Application.
//
// Commits the transaction block to the chains.
func (app *Accumulator) Commit() (resp abci.ResponseCommit) {
	//end the current batch of transactions in the Stateful Merkle Tree

	mdRoot, err := app.chain.Commit()
	resp.Data = mdRoot

	if err != nil {
		//should never get here.
		panic(err)
	}

	//this will truncate what tendermint stores since we only care about current state
	//todo: uncomment the next line when we have smt state syncing complete. For now, we are retaining everything for test net
	// if app.RetainBlocks > 0 && app.Height >= app.RetainBlocks {
	// 	resp.RetainHeight = app.Height - app.RetainBlocks + 1
	// }

	//save the state
	app.state.Size += app.txct
	app.state.AppHash = mdRoot
	app.state.Height++
	saveState(app.state)

	duration := time.Since(app.timer)
	fmt.Printf("%d transactions in %f seconds for a TPS of %f\n", app.txct, duration.Seconds(), float64(app.txct)/duration.Seconds())

	return resp
}

// ListSnapshots implements github.com/tendermint/tendermint/abci/types.Application.
func (app *Accumulator) ListSnapshots(
	req abci.RequestListSnapshots) abci.ResponseListSnapshots {

	req.ProtoMessage()
	return abci.ResponseListSnapshots{}
}

// OfferSnapshot implements github.com/tendermint/tendermint/abci/types.Application.
func (app *Accumulator) OfferSnapshot(
	req abci.RequestOfferSnapshot) abci.ResponseOfferSnapshot {
	return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ABORT}
}

// LoadSnapshotChunk implements github.com/tendermint/tendermint/abci/types.Application.
func (app *Accumulator) LoadSnapshotChunk(
	req abci.RequestLoadSnapshotChunk) abci.ResponseLoadSnapshotChunk {

	//req.Height
	//resp := abcitypes.ResponseLoadSnapshotChunk{}
	//need to get a block of data between markers.
	//resp.Chunk = app.mm.GetState(req.Height)
	return abci.ResponseLoadSnapshotChunk{}
}

// ApplySnapshotChunk implements github.com/tendermint/tendermint/abci/types.Application.
func (app *Accumulator) ApplySnapshotChunk(
	req abci.RequestApplySnapshotChunk) abci.ResponseApplySnapshotChunk {
	return abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ABORT}
}

//updateValidator add, update, or remove a validator
func (app *Accumulator) updateValidator(v abci.ValidatorUpdate) {
	pubkey, _ := encoding.PubKeyFromProto(v.PubKey)

	fmt.Printf("Val Pub Key 0x%X\n", pubkey.Address())
	/*
	   	if err != nil {
	   		panic(fmt.Errorf("can't decode public key: %w", err))
	   	}
	   	//key := []byte("val:" + string(pubkey.Bytes()))
	   	if v.Power == 0 {
	   		// remove validator
	   		_, found := app.tmvalidators[string(pubkey.Address())]// app.app.state.db.Has(key)
	   		if !found {
	   			pubStr := base64.StdEncoding.EncodeToString(pubkey.Bytes())
	   			return abcitypes.ResponseDeliverTx{
	   				Code: code.CodeTypeUnauthorized,
	   				Log:  fmt.Sprintf("Cannot remove non-existent validator %s", pubStr)}
	   		}
	   //		if !hasKey
	   		//if err = app.app.state.db.Delete(key); err != nil {
	   		//	panic(err)
	   		//}
	   		delete(app.tmvalidators, string(pubkey.Address()))
	   	} else {
	   		// add or update validator
	   		//value := bytes.NewBuffer(make([]byte, 0))
	   		//if err := types.WriteMessage(&v, value); err != nil {
	   		//	return types.ResponseDeliverTx{
	   		//		Code: code.CodeTypeEncodingError,
	   		//		Log:  fmt.Sprintf("Error encoding validator: %v", err)}
	   		//}
	   		//if err = app.app.state.db.Set(key, value.Bytes()); err != nil {
	   		//	panic(err)
	   		//}
	   		app.tmvalidators[string(pubkey.Address())] = pubkey
	   	}
	*/

	// we only update the changes array if we successfully updated the tree
	//app.ValUpdates = append(app.ValUpdates, v)
}