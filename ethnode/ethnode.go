package ethnode

import (
	"strconv"

	jsonrpc2 "github.com/ybbus/jsonrpc"
)

// GetBlockNumber returns the block number in the response of a `eth_blockNumber` call.
func GetBlockNumber(nodeURL string) (int64, error) {
	rpc := jsonrpc2.NewClient(nodeURL)
	rep, err := rpc.Call("eth_blockNumber")
	if err != nil {
		return 0, err
	}
	hex, err := rep.GetString()
	if err != nil {
		return 0, err
	}
	dec, err := strconv.ParseInt(hex, 0, 64)
	if err != nil {
		return 0, err
	}
	return dec, nil
}
