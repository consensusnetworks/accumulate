package protocol

import (
	"fmt"

	"github.com/AccumulateNetwork/accumulate/types"
	"github.com/AccumulateNetwork/accumulate/types/state"
)

func NewChain(typ types.ChainType) (state.Chain, error) {
	switch typ {
	case types.ChainTypeIdentity:
		return new(state.AdiState), nil
	case types.ChainTypeTokenIssuer:
		return new(TokenIssuer), nil
	case types.ChainTypeTokenAccount:
		return new(state.TokenAccount), nil
	case types.ChainTypeLiteTokenAccount:
		return new(LiteTokenAccount), nil
	case types.ChainTypeTransactionReference:
		return new(state.TxReference), nil
	case types.ChainTypeTransaction:
		return new(state.Transaction), nil
	case types.ChainTypePendingTransaction:
		return new(state.PendingTransaction), nil
	case types.ChainTypeKeyPage:
		return new(KeyPage), nil
	case types.ChainTypeKeyBook:
		return new(KeyBook), nil
	case types.ChainTypeDataAccount:
		return new(DataAccount), nil
	case types.ChainTypeLiteDataAccount:
		return new(LiteDataAccount), nil
	case types.ChainTypeSyntheticTransactions:
		return new(state.SyntheticTransactionChain), nil
	default:
		return nil, fmt.Errorf("unknown chain type %v", typ)
	}
}

func UnmarshalChain(data []byte) (state.Chain, error) {
	header := new(state.ChainHeader)
	err := header.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}

	chain, err := NewChain(header.Type)
	if err != nil {
		return nil, err
	}

	err = chain.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}

	return chain, nil
}
