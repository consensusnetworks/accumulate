package chain

import (
	"fmt"

	"github.com/AccumulateNetwork/accumulate/internal/url"
	"github.com/AccumulateNetwork/accumulate/protocol"
	"github.com/AccumulateNetwork/accumulate/types"
	"github.com/AccumulateNetwork/accumulate/types/api/transactions"
	"github.com/AccumulateNetwork/accumulate/types/state"
)

type SyntheticDepositTokens struct{}

func (SyntheticDepositTokens) Type() types.TxType {
	return types.TxTypeSyntheticDepositTokens
}

func (SyntheticDepositTokens) Validate(st *StateManager, tx *transactions.GenTransaction) error {
	// *big.Int, tokenChain, *url.URL
	body := new(protocol.SyntheticDepositTokens)
	err := tx.As(body)
	if err != nil {
		return fmt.Errorf("invalid payload: %v", err)
	}

	accountUrl, err := url.Parse(tx.SigInfo.URL)
	if err != nil {
		return fmt.Errorf("invalid recipient URL: %v", err)
	}

	tokenUrl, err := url.Parse(body.Token)
	if err != nil {
		return fmt.Errorf("invalid token URL: %v", err)
	}

	var account tokenChain
	if st.Origin != nil {
		switch origin := st.Origin.(type) {
		case *protocol.LiteTokenAccount:
			account = origin
		case *state.TokenAccount:
			account = origin
		default:
			return fmt.Errorf("invalid origin record: want chain type %v or %v, got %v", types.ChainTypeLiteTokenAccount, types.ChainTypeTokenAccount, origin.Header().Type)
		}
	} else if keyHash, tok, err := protocol.ParseLiteAddress(accountUrl); err != nil {
		return fmt.Errorf("invalid lite token account URL: %v", err)
	} else if keyHash == nil {
		return fmt.Errorf("could not find token account")
	} else if !tokenUrl.Equal(tok) {
		return fmt.Errorf("token URL does not match lite token account URL")
	} else {
		// Address is lite and the account doesn't exist, so create one
		lite := protocol.NewLiteTokenAccount()
		lite.ChainUrl = types.String(accountUrl.String())
		lite.TokenUrl = tokenUrl.String()
		account = lite
	}

	if !account.CreditTokens(&body.Amount) {
		return fmt.Errorf("unable to add deposit balance to account")
	}
	st.Update(account)

	//create a transaction reference chain acme-xxxxx/0, 1, 2, ... n.
	//This will reference the txid to keep the history
	txHash := types.Bytes(tx.TransactionHash()).AsBytes32()
	refUrl := accountUrl.JoinPath(fmt.Sprint(account.NextTx()))
	txr := state.NewTxReference(refUrl.String(), txHash[:])
	st.Update(txr)

	return nil
}
