package main

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mikispag/eth-node-healthcheck/ethnode"
	"github.com/mikispag/eth-node-healthcheck/web"
	log "github.com/sirupsen/logrus"
)

const (
	blockCypherURL = "https://api.blockcypher.com/v1/eth/main"
	etherscanURL   = "https://api.etherscan.io/api?module=proxy&action=eth_blockNumber"
	nanoPoolURL    = "https://api.nanopool.org/v1/eth/network/lastblocknumber/"
)

func main() {
	node := flag.String("node", "http://localhost:8545", "the URL of the local Ethereum node")
	port := flag.Int("port", 8500, "the HTTP port on which to listen")
	threshold := flag.Int64("threshold", 10, "the maximum acceptable number of blocks to allow the node to be behind")
	flag.Parse()

	// Initialize logger
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC822,
	})
	log.SetLevel(log.DebugLevel)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var blockCypherHeight int64
		var nanoPoolHeight int64
		var etherscanHeight int64
		var localHeight int64
		var j map[string]interface{}

		// Query BlockCypher
		j, err := web.GetJSON(blockCypherURL)
		if err != nil {
			log.WithError(err).Error("Unable to read from BlockCypher API!")
			// Continue!
		} else {
			if h, ok := j["height"].(float64); ok {
				blockCypherHeight = int64(h)
			} else {
				log.Error("Unable to read block height from the BlockCypher API response: %#v!", j)
				// Continue!
			}
		}

		// Query NanoPool
		j, err = web.GetJSON(nanoPoolURL)
		if err != nil {
			log.WithError(err).Error("Unable to read from NanoPool API!")
			// Continue!
		} else {
			if h, ok := j["data"].(float64); ok {
				nanoPoolHeight = int64(h)
			} else {
				log.Error("Unable to read block height from the NanoPool API response: %#v!", j)
				// Continue!
			}
		}

		// Query Etherscan
		j, err = web.GetJSON(etherscanURL)
		if err != nil {
			log.WithError(err).Error("Unable to read from Etherscan API!")
			// Continue!
		} else {
			if hex, ok := j["result"].(string); ok {
				etherscanHeight, err = strconv.ParseInt(hex, 0, 64)
				if err != nil {
					log.WithError(err).Errorf("Unable to convert hexadecimal block number to integer from the Etherscan API response: %s!", hex)
					// Continue!
				}
			} else {
				log.Errorf("Unable to read block height from the Etherscan API response: %#v!", j)
				// Continue!
			}
		}

		// Query the local node over JSON-RPC
		localHeight, err = ethnode.GetBlockNumber(*node)
		if err != nil {
			log.WithError(err).Error("JSON-RPC request to the local node failed!")
			http.Error(w, "JSON-RPC request to the local node failed!", 400)
			return
		}

		// Print heights
		log.WithFields(log.Fields{
			"localHeight":       localHeight,
			"blockCypherHeight": blockCypherHeight,
			"nanoPoolHeight":    nanoPoolHeight,
			"etherscanHeight":   etherscanHeight,
		}).Info("Queried heights.", localHeight)

		// Check heights
		localHeightPlusThreshold := localHeight + *threshold
		// Compare the local height plus threshold against the maximum block height received from external sources
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
		heightDiff := maxExternalHeight - localHeightPlusThreshold
		if heightDiff <= 0 {
			log.Info("The local node is fully in sync.")
			w.Write([]byte("The local node is fully in sync."))
		} else {
			log.Warnf("The local node is %d blocks behind!", heightDiff)
			http.Error(w, fmt.Sprintf("The local node is %d blocks behind!", heightDiff), 400)
			return
		}
	})
	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(*port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  25 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
