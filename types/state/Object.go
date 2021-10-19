package state

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"

	"github.com/AccumulateNetwork/accumulated/types"
)

//maybe we should have Chain header then entry, rather than entry containing all the Headers
type Object struct {
	MDRoot types.Bytes32 `json:"pendingMDRoot"`
	Entry  types.Bytes   `json:"stateEntry"` //this is the state data that stores the current state of the chain
}

func (app *Object) MarshalBinary() ([]byte, error) {
	var buffer bytes.Buffer

	buffer.Write(app.MDRoot.Bytes())

	data, err := app.Entry.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buffer.Write(data)

	return buffer.Bytes(), nil
}

func (app *Object) UnmarshalBinary(data []byte) (err error) {
	defer func() {
		if rErr := recover(); rErr != nil {
			err = fmt.Errorf("error unmarshaling Chain State Object, %v", rErr)
		}
	}()

	//minimum length of a chain header is 33 bytes
	if len(data) < 33 {
		return fmt.Errorf("insufficicient data associated with state entry")
	}

	app.MDRoot.FromBytes(data)

	err = app.Entry.UnmarshalBinary(data[32:])
	if err != nil {
		return fmt.Errorf("no state object associated with state entry, %v", err)
	}

	return nil
}

func (o *Object) As(entry encoding.BinaryUnmarshaler) error {
	return entry.UnmarshalBinary(o.Entry)
}

type StateEntry struct {
	ChainState  *Object
	ChainId     *types.Bytes32
	ChainHeader *ChainHeader

	DB *StateDB
}

// LoadChainState retrieves the specified chain and unmarshals it, and
// retrieves its ADI and unmarshals it.
func (s *StateDB) LoadChainState(chainId []byte) (*StateEntry, error) {
	chain32 := types.Bytes(chainId).AsBytes32()
	chainState, chainHeader, err := s.LoadChain(chainId)
	if errors.Is(err, ErrNotFound) {
		return &StateEntry{
			DB:      s,
			ChainId: &chain32,
		}, nil
	} else if err != nil {
		return nil, err
	}

	return &StateEntry{
		DB:          s,
		ChainId:     &chain32,
		ChainState:  chainState,
		ChainHeader: chainHeader,
	}, nil
}
