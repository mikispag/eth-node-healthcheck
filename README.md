# eth-node-healthcheck
A simple Ethereum node health check service written in Go.

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
