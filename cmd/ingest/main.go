package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"soroban-governor-backend/internal/governor"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/ingest/ledgerbackend"
	"github.com/stellar/go/network"
	"github.com/stellar/go/xdr"
	"github.com/stellar/stellar-rpc/client"
)

func main() {
	ctx := context.Background()

	// Use the public SDF Testnet RPC for demo purpose
	endpoint := "https://soroban-testnet.stellar.org"

	// Create a new RPC client
	rpcClient := client.NewClient(endpoint, nil)

	// Get the latest ledger sequence from the RPC server
	health, err := rpcClient.GetHealth(ctx)
	if err != nil {
		log.Fatalf("Failed to get RPC health: %v", err)
	}
	startSeq := health.LatestLedger

	// TEMP - created 1158300

	// Fetch the most recently processed ledger, if any.

	//!! TODO

	fmt.Println("Iterating over ledgers:")

	// Configure the RPC Ledger Backend
	backend := ledgerbackend.NewRPCLedgerBackend(ledgerbackend.RPCLedgerBackendOptions{
		RPCServerURL: endpoint,
	})
	defer backend.Close()

	fmt.Printf("Prepare unbounded range starting with Testnet ledger sequence %d: \n", startSeq)
	// Prepare an unbounded range starting from the latest ledger
	if err := backend.PrepareRange(ctx, ledgerbackend.UnboundedRange(startSeq)); err != nil {
		log.Fatalf("Failed to prepare range: %v", err)
	}

	fmt.Println("Setup complete!")
	fmt.Println("Iterating over ledgers:")
	seq := startSeq
	for {
		fmt.Printf("Fetching ledger %d...\n", seq)
		ledger, err := backend.GetLedger(ctx, seq)
		if err != nil {
			fmt.Printf("No more ledgers or error at sequence %d: %v\n", seq, err)
			break
		}

		txReader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(network.TestNetworkPassphrase, ledger)
		if err != nil {
			log.Fatalf("Failed to create transaction reader for ledger %d: %v", seq, err)
		}

		// Pull events from the transaction reader
		for {
			tx, err := txReader.Read()
			if err != nil {
				if err == io.EOF {
					break
				} else {
					log.Fatalf("Error reading transaction from ledger %d: %v", seq, err)
				}
			}

			if !tx.Successful() {
				continue
			}

			op_0, ok := tx.GetOperation(0)
			if !ok {
				continue
			}
			if op_0.Body.Type != xdr.OperationTypeInvokeHostFunction {
				continue
			}

			events, err := tx.GetContractEvents()
			if err != nil {
				fmt.Printf("Error getting logs for tx. Hash: %x - %v\n", tx.Hash, err)
			}

			for event_index, event := range events {
				gov_event, err := governor.NewGovernorEventFromContractEvent(&event, tx.Hash.HexString(), ledger.LedgerCloseTime(), ledger.LedgerSequence(), int32(tx.Index), 0, int32(event_index))
				byteStr, binErr := event.MarshalBinary()
				if binErr != nil {
					fmt.Printf("Unable to marshal event for logging: %v, %v\n", binErr, event)
					continue
				}
				eventStr := base64.StdEncoding.EncodeToString(byteStr)
				if err != nil {
					// only log failures for events we attempt to parse
					if errors.Is(err, governor.ErrEventParsingFailed) {
						fmt.Printf("Failed parsing event: %v %s\n", err, eventStr)
					} else {
						fmt.Printf("Untracked event: %v %s\n", err, eventStr)
					}
					continue
				}

				fmt.Printf("Do something with this: %v\n", gov_event)
				fmt.Printf("Event: %s\n", eventStr)
				// Process event
			}
		}

		// store most recently completed ledger
		fmt.Printf("Ledger %d processed.\n\n", ledger.LedgerSequence())
		seq++
	}

	fmt.Println("Done.")
}
