package chain_test

import (
	"fmt"
	"testing"

	. "github.com/AccumulateNetwork/accumulate/internal/chain"
	testing2 "github.com/AccumulateNetwork/accumulate/internal/testing"
	"github.com/AccumulateNetwork/accumulate/protocol"
	"github.com/AccumulateNetwork/accumulate/smt/storage"
	"github.com/AccumulateNetwork/accumulate/types"
	"github.com/AccumulateNetwork/accumulate/types/state"
	"github.com/stretchr/testify/require"
)

func TestSynthTokenDeposit_Lite(t *testing.T) {
	tokenUrl := protocol.AcmeUrl().String()

	_, _, gtx, err := testing2.BuildTestSynthDepositGenTx()
	require.NoError(t, err)

	db := new(state.StateDB)
	require.NoError(t, db.Open("mem", true, true, nil))

	st, err := NewStateManager(db.Begin(), gtx)
	require.ErrorIs(t, err, storage.ErrNotFound)

	err = SyntheticDepositTokens{}.Validate(st, gtx)
	require.NoError(t, err)

	//try to extract the state to see if we have a valid account
	tas := new(protocol.LiteTokenAccount)
	require.NoError(t, st.LoadAs(st.OriginChainId, tas))
	require.Equal(t, types.String(gtx.SigInfo.URL), tas.ChainUrl, "invalid chain header")
	require.Equalf(t, types.ChainTypeLiteTokenAccount, tas.Type, "chain state is not a lite account, it is %s", tas.ChainHeader.Type.Name())
	require.Equal(t, tokenUrl, tas.TokenUrl, "token url of state doesn't match expected")
	require.Equal(t, uint64(1), tas.TxCount)

	//now query the tx reference
	refUrl := st.OriginUrl.JoinPath(fmt.Sprint(tas.TxCount - 1))
	txRef := new(state.TxReference)
	require.NoError(t, st.LoadUrlAs(refUrl, txRef))
	require.Equal(t, types.String(refUrl.String()), txRef.ChainUrl, "chain header expected transaction reference")
	require.Equal(t, gtx.TransactionHash(), txRef.TxId[:], "txid doesn't match")
}
