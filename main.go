package main

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mikispag/eth-node-healthcheck/ethnode"
	"github.com/mikispag/utils/web"
	log "github.com/sirupsen/logrus"
)

const (
	blockCypherURL = "https://api.blockcypher.com/v1/eth/main"
	etherscanURL   = "https://api.etherscan.io/api?module=proxy&action=eth_blockNumber"
	nanoPoolURL    = "https://api.nanopool.org/v1/eth/network/lastblocknumber/"
)

var (
	node      = flag.String("node", "http://localhost:8545", "the URL of the Ethereum node to check for health")
	port      = flag.Int("port", 8500, "the HTTP port on which to listen")
	timeout   = flag.Duration("timeout", 30*time.Second, "the timeout for the entire check routine")
	threshold = flag.Int64("threshold", 10, "the maximum acceptable number of blocks to allow the node to be behind")
	webClient = web.NewWithTimeout(*timeout / 3)
)

func handler(w http.ResponseWriter, r *http.Request) {
	var nodeHeight int64
	// Query the node over JSON-RPC
	nodeHeight, err := ethnode.GetBlockNumber(*node)
	if err != nil {
		log.WithError(err).Error("JSON-RPC request to the node failed!")
		http.Error(w, "JSON-RPC request to the node failed!", http.StatusServiceUnavailable)
		return
	}
	log.WithField("height", nodeHeight).Debug("Node queried.")

	// Query BlockCypher
	blockCypherChannel := make(chan int64)
	go func() {
		var j map[string]interface{}

		err = webClient.GetJSON(blockCypherURL, &j)
		if err != nil {
			log.WithError(err).Error("Unable to read from BlockCypher API!")
			blockCypherChannel <- 0
			return
		}
		if h, ok := j["height"].(float64); ok {
			height := int64(h)
			log.WithField("height", height).Debug("BlockCypher queried.")
			blockCypherChannel <- height
		} else {
			log.Errorf("Unable to read block height from the BlockCypher API response: %#v!", j)
			blockCypherChannel <- 0
		}
	}()

	// Query NanoPool
	nanoPoolChannel := make(chan int64)
	go func() {
		var j map[string]interface{}

		err = webClient.GetJSON(nanoPoolURL, &j)
		if err != nil {
			log.WithError(err).Error("Unable to read from NanoPool API!")
			nanoPoolChannel <- 0
			return
		}
		if h, ok := j["data"].(float64); ok {
			height := int64(h)
			log.WithField("height", height).Debug("NanoPool queried.")
			nanoPoolChannel <- height
		} else {
			log.Errorf("Unable to read block height from the NanoPool API response: %#v!", j)
			nanoPoolChannel <- 0
		}
	}()

	// Query Etherscan
	etherscanChannel := make(chan int64)
	go func() {
		var j map[string]interface{}

		err = webClient.GetJSON(etherscanURL, &j)
		if err != nil {
			log.WithError(err).Error("Unable to read from Etherscan API!")
			etherscanChannel <- 0
			return
		}
		if hex, ok := j["result"].(string); ok {
			etherscanHeight, err := strconv.ParseInt(hex, 0, 64)
			if err != nil {
				log.WithError(err).Errorf("Unable to convert hexadecimal block number to integer from the Etherscan API response: %s!", hex)
				etherscanChannel <- 0
				// Continue!
			} else {
				log.WithField("height", etherscanHeight).Debug("Etherscan queried.")
				etherscanChannel <- etherscanHeight
			}
		} else {
			log.Errorf("Unable to read block height from the Etherscan API response: %#v!", j)
			etherscanChannel <- 0
		}
	}()

	var maxExternalHeight int64
	timeoutChannel := time.After(*timeout)
out:
	for i := 0; i < 3; i++ {
		select {
		case blockCypherHeight := <-blockCypherChannel:
			if blockCypherHeight > maxExternalHeight {
				maxExternalHeight = blockCypherHeight
			}
		case nanoPoolHeight := <-nanoPoolChannel:
			if nanoPoolHeight > maxExternalHeight {
				maxExternalHeight = nanoPoolHeight
			}
		case etherscanHeight := <-etherscanChannel:
			if etherscanHeight > maxExternalHeight {
				maxExternalHeight = etherscanHeight
			}
		case <-timeoutChannel:
			log.Error("Timeout reached while waiting for external block heights.")
			break out
		}
	}

	// If no external heights was fetched, error out.
	if maxExternalHeight == 0 {
		log.Error("No external height was fetched, returning error!")
		http.Error(w, "No external height was fetched, returning error!", http.StatusServiceUnavailable)
		return
	}

	// Check heights
	nodeHeightPlusThreshold := nodeHeight + *threshold
	heightDiff := maxExternalHeight - nodeHeightPlusThreshold
	if heightDiff <= 0 {
		log.Info("The node is fully in sync.")
		_, err := w.Write([]byte("The node is fully in sync."))
		if err != nil {
			log.WithError(err).Error("Unable to write response.")
		}
	} else {
		log.Warnf("The node is %d blocks behind!", heightDiff)
		http.Error(w, fmt.Sprintf("The node is %d blocks behind!", heightDiff), http.StatusServiceUnavailable)
		return
	}
}

func main() {
	flag.Parse()

	// Initialize logger
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	log.SetLevel(log.DebugLevel)

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(*port),
		ReadTimeout:  *timeout,
		WriteTimeout: *timeout + 10*time.Second,
		IdleTimeout:  *timeout * 10,
		Handler:      http.TimeoutHandler(http.HandlerFunc(handler), *timeout, "Timeout reached!"),
	}
	log.Fatal(srv.ListenAndServe())
}
