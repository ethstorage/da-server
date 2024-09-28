package da

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func TestHashEncDec(t *testing.T) {
	h := common.Hash{'a'}
	hBytes := h.Hex()
	comm, err := hexutil.Decode(hBytes)
	require.NoError(t, err)
	hBack := common.BytesToHash(comm)

	require.Equal(t, h, hBack)

}
