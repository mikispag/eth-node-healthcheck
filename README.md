# eth-node-healthcheck
A simple Ethereum node healthcheck service written in Go.

## Usage

```bash
$ eth-node-healthcheck -help
Usage of eth-node-healthcheck:
  -node string
    	the URL of the local Ethereum node (default "http://localhost:8545")
  -port int
    	the HTTP port on which to listen (default 8500)
  -threshold int
    	the maximum acceptable number of blocks to allow the node to be behind (default 10)
```
