package validator

import (
	"encoding/json"
	"github.com/AccumulateNetwork/accumulated/types/api"
	"github.com/AccumulateNetwork/accumulated/types/state"

	//"crypto/sha256"
	"github.com/AccumulateNetwork/accumulated/types"
	"github.com/AccumulateNetwork/accumulated/types/proto"
	"testing"
	"time"
)

func createTokenIssuanceSubmission(t *testing.T, adiChainPath string) (*state.StateEntry, *proto.Submission) {
	kp := types.CreateKeyPair()
	identityhash := types.GetIdentityChainFromIdentity(&adiChainPath).Bytes()

	currentstate := state.StateEntry{}

	//currentstate.ChainState = CreateFakeTokenAccountState(identitychainpath,t)

	currentstate.IdentityState, identityhash = CreateFakeIdentityState(adiChainPath, kp)

	chainid := types.GetChainIdFromChainPath(&adiChainPath).Bytes()

	ti := api.NewToken(adiChainPath, "ACME", 1)

	//build a submission message
	sub := proto.Submission{}
	sub.Data, _ = json.Marshal(ti)

	sub.Instruction = proto.AccInstruction_Token_Issue
	sub.Chainid = chainid[:]
	sub.Identitychain = identityhash[:]
	sub.Timestamp = time.Now().Unix()

	return &currentstate, &sub
}

func TestTokenIssuanceValidator_Check(t *testing.T) {
	tiv := NewTokenIssuanceValidator()
	identitychainpath := "RoadRunner/ACME"
	currentstate, sub := createTokenIssuanceSubmission(t, identitychainpath)

	err := tiv.Check(currentstate, sub.Identitychain, sub.Chainid, 0, 0, sub.Data)

	if err != nil {
		t.Fatal(err)
	}
}

func TestTokenIssuanceValidator_Validate(t *testing.T) {
	//	kp := types.CreateKeyPair()
	tiv := NewTokenIssuanceValidator()
	identitychainpath := "RoadRunner/ACME"
	currentstate, sub := createTokenIssuanceSubmission(t, identitychainpath)

	resp, err := tiv.Validate(currentstate, sub)

	if err != nil {
		t.Fatal(err)
	}

	if resp.Submissions != nil {
		t.Fatalf("expecting no synthetic transactions")
	}

	if resp.StateData == nil {
		t.Fatal("expecting a state object to be returned to add to a token coinbase chain")
	}

	ti := state.Token{}
	chainid := types.GetChainIdFromChainPath(&identitychainpath)
	if resp.StateData == nil {
		t.Fatal("expecting state object from token transaction")
	}

	val, ok := resp.StateData[*chainid]
	if !ok {
		t.Fatalf("token transaction account chain not found %s", identitychainpath)
	}
	err = ti.UnmarshalBinary(val)

	if err != nil {
		t.Fatal(err)
	}
}
