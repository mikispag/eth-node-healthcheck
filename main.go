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
	log.Debug("Querying the node over JSON-RPC...")
	nodeHeight, err := ethnode.GetBlockNumber(*node)
	if err != nil {
		log.WithError(err).Error("JSON-RPC request to the node failed!")
		http.Error(w, "JSON-RPC request to the node failed!", 503)
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
			// Continue!
		} else {
			if h, ok := j["height"].(float64); ok {
				blockCypherChannel <- int64(h)
			} else {
				log.Error("Unable to read block height from the BlockCypher API response: %#v!", j)
				// Continue!
			}
		}
	}()

	// Query NanoPool
	nanoPoolChannel := make(chan int64)
	go func() {
		var j map[string]interface{}

		log.Debug("Querying NanoPool...")
		err = webClient.GetJSON(nanoPoolURL, &j)
		if err != nil {
			log.WithError(err).Error("Unable to read from NanoPool API!")
			// Continue!
		} else {
			if h, ok := j["data"].(float64); ok {
				nanoPoolChannel <- int64(h)
			} else {
				log.Error("Unable to read block height from the NanoPool API response: %#v!", j)
				// Continue!
			}
		}
	}()

	// Query Etherscan
	etherscanChannel := make(chan int64)
	go func() {
		var j map[string]interface{}

		log.Debug("Querying Etherscan...")
		err = webClient.GetJSON(etherscanURL, &j)
		if err != nil {
			log.WithError(err).Error("Unable to read from Etherscan API!")
			// Continue!
		} else {
			if hex, ok := j["result"].(string); ok {
				etherscanHeight, err := strconv.ParseInt(hex, 0, 64)
				if err != nil {
					log.WithError(err).Errorf("Unable to convert hexadecimal block number to integer from the Etherscan API response: %s!", hex)
					// Continue!
				} else {
					etherscanChannel <- etherscanHeight
				}
			} else {
				log.Errorf("Unable to read block height from the Etherscan API response: %#v!", j)
				// Continue!
			}
		}
	}()

	var blockCypherHeight int64
	var nanoPoolHeight int64
	var etherscanHeight int64
	select {
	case blockCypherHeight = <-blockCypherChannel:
		log.WithField("height", blockCypherHeight).Debug("BlockCypher queried.")
	case nanoPoolHeight = <-nanoPoolChannel:
		log.WithField("height", nanoPoolHeight).Debug("NanoPool queried.")
	case etherscanHeight = <-etherscanChannel:
		log.WithField("height", etherscanHeight).Debug("Etherscan queried.")
	case <-time.After(*timeout):
		log.Error("Timeout reached while waiting for external block heights.")
		http.Error(w, "Timeout reached while waiting for external block heights.", 503)
		return
	}

	// Print heights
	log.WithFields(log.Fields{
		"nodeHeight":        nodeHeight,
		"blockCypherHeight": blockCypherHeight,
		"nanoPoolHeight":    nanoPoolHeight,
		"etherscanHeight":   etherscanHeight,
	}).Info("Queried heights.")

	// Check heights
	nodeHeightPlusThreshold := nodeHeight + *threshold
	// Compare the node height plus threshold against the maximum block height received from external sources
	var maxExternalHeight int64
	if blockCypherHeight > maxExternalHeight {
		maxExternalHeight = blockCypherHeight
	}
	if nanoPoolHeight > maxExternalHeight {
		maxExternalHeight = nanoPoolHeight
	}
	if etherscanHeight > maxExternalHeight {
		maxExternalHeight = etherscanHeight
	}
	// If no external heights was fetched, error out.
	if maxExternalHeight == 0 {
		log.Error("No external height was fetched, returning error!")
		http.Error(w, "No external height was fetched, returning error!", 503)
		return
	}

	heightDiff := maxExternalHeight - nodeHeightPlusThreshold
	if heightDiff <= 0 {
		log.Info("The node is fully in sync.")
		w.Write([]byte("The node is fully in sync."))
	} else {
		log.Warnf("The node is %d blocks behind!", heightDiff)
		http.Error(w, fmt.Sprintf("The node is %d blocks behind!", heightDiff), 503)
		return
	}
}

func main() {
	flag.Parse()

	// Initialize logger
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC822,
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
