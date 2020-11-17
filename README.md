# eth-node-healthcheck
A simple Ethereum node health check service written in Go.
It returns a `200` HTTP status code when the monitored node is in sync with the Ethereum mainnet and `503` otherwise. As external oracles, it attempts a consensus between BlockCypher, NanoPool and Etherscan.

## Usage

```bash
$ eth-node-healthcheck -help
Usage of eth-node-healthcheck:
  -node string
    	the URL of the Ethereum node to check for health (default "http://localhost:8545")
  -port int
    	the HTTP port on which to listen (default 8500)
  -threshold int
    	the maximum acceptable number of blocks to allow the node to be behind (default 10)
```
