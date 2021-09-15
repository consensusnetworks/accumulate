package proto

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"

	"github.com/AccumulateNetwork/accumulated/types"

	"github.com/AccumulateNetwork/SMT/common"
)

// GenTransaction
// Every transaction that goes through the Accumulate protocol is packaged
// as a GenTransaction.  This means we implement this once, and most of the
// transaction validation and processing is done in one and only one way.
type GenTransaction struct {
	Signature   []*ED25519Sig // Signature(s) of the transaction
	Routing     uint64        // The first 8 bytes of the hash of the identity
	ChainID     []byte        // The hash of the chain URL
	Transaction []byte        // The transaction that follows
}

func (t *GenTransaction) Equal(t2 *GenTransaction) bool {
	isEqual := true
	for i, sig := range t.Signature {
		isEqual = isEqual && sig.Equal(t2.Signature[i])
	}
	return isEqual &&
		t.Routing == t2.Routing &&
		bytes.Equal(t.ChainID, t2.ChainID) &&
		bytes.Equal(t.Transaction, t2.Transaction)
}

func (t *GenTransaction) SetRoutingChainID(destURL string) error {
	u, err := url.Parse(destURL)
	if err != nil {
		return err
	}
	host := strings.ToLower(u.Host)
	h := sha256.Sum256([]byte(host))
	t.Routing = uint64(h[0])<<56 | uint64(h[1])<<48 | uint64(h[2])<<40 |
		uint64(h[3])<<32 | uint64(h[4])<<24 | uint64(h[5])<<16 |
		uint64(h[6])<<8 | uint64(h[7])
	t.ChainID = types.GetChainIdFromChainPath(&destURL)[:]
	return nil
}

// TxId
// Returns the transaction hash which serves as the identifier for this transaction
func (t *GenTransaction) TxId() []byte {
	if data, err := t.MarshalBinary(); err != nil {
		return nil
	} else {
		h := sha256.Sum256(data)
		return h[:]
	}
}

// ValidateSig
// We validate the signature of the transaction.
func (t *GenTransaction) ValidateSig() bool {
	if h, err := t.MarshalBinary(); err != nil {
		return false
	} else {
		for _, v := range t.Signature {
			if !v.Verify(h[:]) {
				return false
			}
		}
	}
	return true
}

// MarshalBinary
// Marshal the portion of the transaction that must be hashed then signed
func (t *GenTransaction) MarshalBinary() (data []byte, err error) {
	defer func() {
		if err := recover(); err != nil {
			err = fmt.Errorf("error MarshalBinary genTransaction %v", err)
		}
	}()
	data = append(data, common.Uint64Bytes(t.Routing)...)
	data = append(data, common.SliceBytes(t.ChainID)...)
	data = append(data, common.SliceBytes(t.Transaction)...)

	return data, nil
}

func (t *GenTransaction) UnmarshalBinary(data []byte) (nextData []byte, err error) {
	defer func() {
		if err := recover(); err != nil {
			err = fmt.Errorf("error unmarshaling ED25519Sig %v", err)
		}
	}()
	t.Routing, data = common.BytesUint64(data)
	t.ChainID, data = common.BytesSlice(data)
	t.Transaction, data = common.BytesSlice(data)
	return data, nil
}

// UnMarshal
// Create the binary representation of the GenTransaction
func (t *GenTransaction) Marshal() (data []byte, err error) {
	defer func() {
		if err := recover(); err != nil {
			err = fmt.Errorf("error marshaling GenTransaction %v", err)
		}
	}()
	sLen := uint64(len(t.Signature))
	if sLen == 0 || sLen > 100 {
		panic("must have 1 to 100 signatures")
	}
	data = common.Uint64Bytes(sLen)
	for _, v := range t.Signature {
		if sig, err := v.Marshal(); err == nil {
			data = append(data, sig...)
		} else {
			return data, err
		}
	}
	if tdata, err := t.MarshalBinary(); err != nil {
		return nil, err
	} else {
		data = append(data, tdata...)
	}
	return data, nil
}

func (t *GenTransaction) UnMarshal(data []byte) (nextData []byte, err error) {
	defer func() {
		if err := recover(); err != nil {
			err = fmt.Errorf("error unmarshaling GenTransaction %v", err)
		}
	}()
	var sLen uint64
	sLen, data = common.BytesUint64(data)
	if sLen < 1 || sLen > 100 {
		panic("signature length out of range")
	}
	for i := uint64(0); i < sLen; i++ {
		sig := new(ED25519Sig)
		if data, err = sig.Unmarshal(data); err != nil {
			return nil, err
		}
		t.Signature = append(t.Signature, sig)
	}
	data, err = t.UnmarshalBinary(data)
	return data, err
}

// GetTransactionType
// Returns the type of the enclosed transaction
func (t *GenTransaction) GetTransactionType() (txType uint64) {
	txType, _ = common.BytesUint64(t.Transaction)
	return txType
}

func (t *GenTransaction) GetRouting() uint64 {
	return t.Routing
}

func (t *GenTransaction) GetChainID() []byte {
	return t.ChainID
}

func (t *GenTransaction) GetSignature() []*ED25519Sig {
	return t.Signature
}