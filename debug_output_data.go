package main

import (
	"fmt"
	"log"

	"bitcoin-indexer/internal/bitcoin"
)

func main() {
	// Test Bitcoin RPC connection
	rpcClient := bitcoin.NewRPCClient(
		"https://gate.bokands.xyz/btc/",
		"xvault",
		"LgwgUaZMDXs2X3hAep9ZpKHcQFszcUy7",
	)

	// Binary search for the first block with multiple transactions
	fmt.Println("🔍 Binary search for first block with multiple transactions...")

	// We know block 220 has 1 transaction, and block 800000 has 3720 transactions
	// So the transition happens somewhere between 220 and 800000
	left := 220
	right := 800000
	firstMultiTxBlock := -1

	for left <= right {
		mid := (left + right) / 2
		fmt.Printf("Checking block %d... ", mid)

		// Get block hash
		blockHash, err := rpcClient.GetBlockHash(mid)
		if err != nil {
			fmt.Printf("Failed to get block hash: %v\n", err)
			break
		}

		// Get block data
		block, err := rpcClient.GetBlock(blockHash)
		if err != nil {
			fmt.Printf("Failed to get block data: %v\n", err)
			break
		}

		txCount := len(block.Transactions)
		fmt.Printf("has %d transactions\n", txCount)

		if txCount > 1 {
			// This block has multiple transactions, search earlier
			firstMultiTxBlock = mid
			right = mid - 1
		} else {
			// This block has only 1 transaction, search later
			left = mid + 1
		}
	}

	if firstMultiTxBlock != -1 {
		fmt.Printf("\n🎯 Found first block with multiple transactions: Block %d\n", firstMultiTxBlock)

		// Check a few blocks around the transition point
		fmt.Println("\n📊 Transaction count around the transition:")
		for i := -3; i <= 3; i++ {
			checkBlock := firstMultiTxBlock + i
			if checkBlock < 0 {
				continue
			}

			blockHash, err := rpcClient.GetBlockHash(checkBlock)
			if err != nil {
				fmt.Printf("Block %d: Error getting block hash: %v\n", checkBlock, err)
				continue
			}

			block, err := rpcClient.GetBlock(blockHash)
			if err != nil {
				fmt.Printf("Block %d: Error getting block data: %v\n", checkBlock, err)
				continue
			}

			fmt.Printf("Block %d: %d transactions\n", checkBlock, len(block.Transactions))
		}

		// Get details of the first multi-transaction block
		blockHash, err := rpcClient.GetBlockHash(firstMultiTxBlock)
		if err != nil {
			log.Fatalf("Failed to get block hash: %v", err)
		}

		block, err := rpcClient.GetBlock(blockHash)
		if err != nil {
			log.Fatalf("Failed to get block data: %v", err)
		}

		fmt.Printf("\n📋 Block %d details:\n", firstMultiTxBlock)
		fmt.Printf("  - Hash: %s\n", block.Hash)
		fmt.Printf("  - Time: %d (Unix timestamp)\n", block.Time)
		fmt.Printf("  - Transactions: %d\n", len(block.Transactions))

		// Show first few transactions
		for i, tx := range block.Transactions {
			if i >= 5 { // Show only first 5 transactions
				fmt.Printf("  - ... and %d more transactions\n", len(block.Transactions)-5)
				break
			}
			isCoinbase := len(tx.Vin) == 1 && tx.Vin[0].Txid == ""
			fmt.Printf("  - Transaction %d: %s (coinbase: %v)\n", i, tx.Txid, isCoinbase)
		}
	} else {
		fmt.Println("❌ Could not find a block with multiple transactions")
	}
}
