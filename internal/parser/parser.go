package parser

import (
	"encoding/hex"
	"fmt"
	"time"

	"bitcoin-indexer/internal/bitcoin"
	"bitcoin-indexer/internal/database"
	"bitcoin-indexer/internal/models"
)

// Parser handles Bitcoin transaction parsing and UTXO tracking
type Parser struct {
	db *database.DB
}

// NewParser creates a new transaction parser
func NewParser(db *database.DB) *Parser {
	return &Parser{db: db}
}

// ParseBlock parses a block and extracts ALL transactions with sender computation for watched addresses
func (p *Parser) ParseBlock(block *bitcoin.Block) ([]*models.UTXO, []*models.UTXO, []*models.TransactionReference, error) {
	var newUTXOs []*models.UTXO
	var spentUTXOs []*models.UTXO
	var txReferences []*models.TransactionReference

	for _, tx := range block.Transactions {
		// Parse outputs (ALL new UTXOs, not just watched ones)
		for _, vout := range tx.Vout {
			// Extract address with priority: address field > addresses[0] > derived identifier
			address := ExtractAddressFromScriptPubKey(vout.ScriptPubKey)

			// Store ALL UTXOs
			utxo := &models.UTXO{
				Txid:        tx.Txid,
				Vout:        vout.N,
				Address:     address,
				ScriptHex:   vout.ScriptPubKey.Hex,
				ValueSats:   int64(vout.Value * 100000000), // Convert BTC to satoshis
				Status:      "confirmed",
				BlockHeight: &block.Height,
				BlockHash:   &block.Hash,
				FirstSeenAt: time.Unix(block.Time, 0),
			}
			newUTXOs = append(newUTXOs, utxo)

			// Check if this output is for a watched address
			isWatched, _, err := p.db.IsScriptWatched(vout.ScriptPubKey.Hex)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to check if script is watched: %w", err)
			}

			if isWatched {
				// Compute sender address for incoming transaction
				senderAddress := p.computeSenderAddress(&tx, block.Height)

				blockTimestamp := int(block.Time)
				txRef := &models.TransactionReference{
					Txid:           tx.Txid,
					Address:        address,
					Direction:      "in",
					ValueSats:      int64(vout.Value * 100000000),
					SenderAddress:  senderAddress,
					BlockHeight:    &block.Height,
					BlockTimestamp: &blockTimestamp,
					CreatedAt:      time.Now(),
				}
				txReferences = append(txReferences, txRef)
			}
		}

		// Parse inputs (ALL spent UTXOs)
		for _, vin := range tx.Vin {
			if vin.Txid != "" && vin.Vout >= 0 { // Skip coinbase inputs
				// Get the spent UTXO to mark it as spent
				existingUTXO, err := p.db.GetUTXO(vin.Txid, vin.Vout)
				if err != nil {
					// UTXO not found in our database, skip
					continue
				}

				if existingUTXO != nil {
					// Mark as spent
					spentUTXO := *existingUTXO
					spentUTXO.Status = "spent"
					spentAt := time.Unix(block.Time, 0)
					spentUTXO.SpentAt = &spentAt
					spentUTXO.SpentByTxid = &tx.Txid
					spentUTXOs = append(spentUTXOs, &spentUTXO)

					// Check if the spent UTXO was for a watched address
					isWatched, _, err := p.db.IsScriptWatched(existingUTXO.ScriptHex)
					if err != nil {
						return nil, nil, nil, fmt.Errorf("failed to check if script is watched: %w", err)
					}

					if isWatched {
						// Compute receiver address for outgoing transaction
						receiverAddress := p.computeReceiverAddress(&tx, block.Height)

						blockTimestamp := int(block.Time)
						txRef := &models.TransactionReference{
							Txid:            tx.Txid,
							Address:         existingUTXO.Address,
							Direction:       "out",
							ValueSats:       existingUTXO.ValueSats,
							ReceiverAddress: receiverAddress,
							BlockHeight:     &block.Height,
							BlockTimestamp:  &blockTimestamp,
							CreatedAt:       time.Now(),
						}
						txReferences = append(txReferences, txRef)
					}
				}
			}
		}
	}

	return newUTXOs, spentUTXOs, txReferences, nil
}

// ParseBlockWithCache parses a block using cached watched scripts for better performance
func (p *Parser) ParseBlockWithCache(block *bitcoin.Block, watchedScriptMap map[string]*models.WatchedScript) ([]*models.UTXO, []*models.UTXO, []*models.TransactionReference, error) {
	var newUTXOs []*models.UTXO
	var spentUTXOs []*models.UTXO
	var txReferences []*models.TransactionReference

	for _, tx := range block.Transactions {
		// Parse outputs (ALL new UTXOs, not just watched ones)
		for _, vout := range tx.Vout {
			// Extract address with priority: address field > addresses[0] > derived identifier
			address := ExtractAddressFromScriptPubKey(vout.ScriptPubKey)

			// Store ALL UTXOs
			utxo := &models.UTXO{
				Txid:        tx.Txid,
				Vout:        vout.N,
				Address:     address,
				ScriptHex:   vout.ScriptPubKey.Hex,
				ValueSats:   int64(vout.Value * 100000000), // Convert BTC to satoshis
				Status:      "confirmed",
				BlockHeight: &block.Height,
				BlockHash:   &block.Hash,
				FirstSeenAt: time.Unix(block.Time, 0),
			}
			newUTXOs = append(newUTXOs, utxo)

			// Check if this output is for a watched address using cache
			if _, isWatched := watchedScriptMap[vout.ScriptPubKey.Hex]; isWatched {
				// Compute sender address for incoming transaction
				senderAddress := p.computeSenderAddress(&tx, block.Height)

				blockTimestamp := int(block.Time)
				txRef := &models.TransactionReference{
					Txid:           tx.Txid,
					Address:        address,
					Direction:      "in",
					ValueSats:      int64(vout.Value * 100000000),
					SenderAddress:  senderAddress,
					BlockHeight:    &block.Height,
					BlockTimestamp: &blockTimestamp,
					CreatedAt:      time.Now(),
				}
				txReferences = append(txReferences, txRef)
			}
		}

		// Parse inputs (ALL spent UTXOs)
		for _, vin := range tx.Vin {
			if vin.Txid != "" && vin.Vout >= 0 { // Skip coinbase inputs
				// Get the spent UTXO to mark it as spent
				existingUTXO, err := p.db.GetUTXO(vin.Txid, vin.Vout)
				if err != nil {
					// UTXO not found in our database, skip
					continue
				}

				if existingUTXO != nil {
					// Mark as spent
					spentUTXO := *existingUTXO
					spentUTXO.Status = "spent"
					spentAt := time.Unix(block.Time, 0)
					spentUTXO.SpentAt = &spentAt
					spentUTXO.SpentByTxid = &tx.Txid
					spentUTXOs = append(spentUTXOs, &spentUTXO)

					// Check if the spent UTXO was for a watched address using cache
					if _, isWatched := watchedScriptMap[existingUTXO.ScriptHex]; isWatched {
						// Compute receiver address for outgoing transaction
						receiverAddress := p.computeReceiverAddress(&tx, block.Height)

						blockTimestamp := int(block.Time)
						txRef := &models.TransactionReference{
							Txid:            tx.Txid,
							Address:         existingUTXO.Address,
							Direction:       "out",
							ValueSats:       existingUTXO.ValueSats,
							ReceiverAddress: receiverAddress,
							BlockHeight:     &block.Height,
							BlockTimestamp:  &blockTimestamp,
							CreatedAt:       time.Now(),
						}
						txReferences = append(txReferences, txRef)
					}
				}
			}
		}
	}

	return newUTXOs, spentUTXOs, txReferences, nil
}

// ParseMempoolTransaction parses a mempool transaction
func (p *Parser) ParseMempoolTransaction(tx *bitcoin.Tx) ([]*models.UTXO, []*models.UTXO, []*models.TransactionReference, error) {
	var newUTXOs []*models.UTXO
	var spentUTXOs []*models.UTXO
	var txReferences []*models.TransactionReference

	// Parse outputs (ALL new UTXOs, not just watched ones)
	for _, vout := range tx.Vout {
		// Extract address with priority: address field > addresses[0] > derived identifier
		address := ExtractAddressFromScriptPubKey(vout.ScriptPubKey)

		// Store ALL UTXOs
		utxo := &models.UTXO{
			Txid:        tx.Txid,
			Vout:        vout.N,
			Address:     address,
			ScriptHex:   vout.ScriptPubKey.Hex,
			ValueSats:   int64(vout.Value * 100000000), // Convert BTC to satoshis
			Status:      "mempool",
			FirstSeenAt: time.Now(),
		}
		newUTXOs = append(newUTXOs, utxo)

		// Check if this output is for a watched address
		isWatched, _, err := p.db.IsScriptWatched(vout.ScriptPubKey.Hex)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to check if script is watched: %w", err)
		}

		if isWatched {
			// Compute sender address for incoming transaction
			senderAddress := p.computeSenderAddress(tx, 0) // 0 for mempool

			txRef := &models.TransactionReference{
				Txid:          tx.Txid,
				Address:       address,
				Direction:     "in",
				ValueSats:     int64(vout.Value * 100000000),
				SenderAddress: senderAddress,
				CreatedAt:     time.Now(),
			}
			txReferences = append(txReferences, txRef)
		}
	}

	// Parse inputs (ALL spent UTXOs)
	for _, vin := range tx.Vin {
		if vin.Txid != "" && vin.Vout >= 0 { // Skip coinbase inputs
			// Get the spent UTXO to mark it as spent
			existingUTXO, err := p.db.GetUTXO(vin.Txid, vin.Vout)
			if err != nil {
				// UTXO not found in our database, skip
				continue
			}

			if existingUTXO != nil {
				// Mark as spent
				spentUTXO := *existingUTXO
				spentUTXO.Status = "spent"
				spentAt := time.Now()
				spentUTXO.SpentAt = &spentAt
				spentUTXO.SpentByTxid = &tx.Txid
				spentUTXOs = append(spentUTXOs, &spentUTXO)

				// Check if the spent UTXO was for a watched address
				isWatched, _, err := p.db.IsScriptWatched(existingUTXO.ScriptHex)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("failed to check if script is watched: %w", err)
				}

				if isWatched {
					// Compute receiver address for outgoing transaction
					receiverAddress := p.computeReceiverAddress(tx, 0) // 0 for mempool

					txRef := &models.TransactionReference{
						Txid:            tx.Txid,
						Address:         existingUTXO.Address,
						Direction:       "out",
						ValueSats:       existingUTXO.ValueSats,
						ReceiverAddress: receiverAddress,
						CreatedAt:       time.Now(),
					}
					txReferences = append(txReferences, txRef)
				}
			}
		}
	}

	return newUTXOs, spentUTXOs, txReferences, nil
}

// ProcessUTXOs processes new and spent UTXOs and transaction references using batch operations
func (p *Parser) ProcessUTXOs(newUTXOs, spentUTXOs []*models.UTXO, txReferences []*models.TransactionReference) error {
	// Process new UTXOs in batch
	if len(newUTXOs) > 0 {
		if err := p.db.BatchUpsertUTXOs(newUTXOs); err != nil {
			return fmt.Errorf("failed to batch upsert UTXOs: %w", err)
		}
	}

	// Process spent UTXOs in batch
	if len(spentUTXOs) > 0 {
		if err := p.db.BatchMarkUTXOsSpent(spentUTXOs); err != nil {
			return fmt.Errorf("failed to batch mark UTXOs as spent: %w", err)
		}
	}

	// Process transaction references in batch
	if len(txReferences) > 0 {
		if err := p.db.BatchUpsertTransactionReferences(txReferences); err != nil {
			return fmt.Errorf("failed to batch upsert transaction references: %w", err)
		}
	}

	return nil
}

// computeSenderAddress computes the sender address from transaction inputs
func (p *Parser) computeSenderAddress(tx *bitcoin.Tx, blockHeight int) *string {
	// For simplicity, we'll use the first input's previous output address as sender
	// In a more sophisticated implementation, you'd analyze the scriptSig
	for _, vin := range tx.Vin {
		if vin.Txid != "" && vin.Vout >= 0 {
			// Get the previous UTXO to find the sender address
			prevUTXO, err := p.db.GetUTXO(vin.Txid, vin.Vout)
			if err == nil && prevUTXO != nil {
				return &prevUTXO.Address
			}
		}
	}
	return nil
}

// computeReceiverAddress computes the receiver address from transaction outputs
func (p *Parser) computeReceiverAddress(tx *bitcoin.Tx, blockHeight int) *string {
	// For simplicity, we'll use the first output address as receiver
	// In a more sophisticated implementation, you'd analyze all outputs
	for _, vout := range tx.Vout {
		address := ExtractAddressFromScriptPubKey(vout.ScriptPubKey)
		if address != "" {
			return &address
		}
	}
	return nil
}

// GetScriptType determines the script type from script hex
func GetScriptType(scriptHex string) (string, error) {
	script, err := hex.DecodeString(scriptHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode script hex: %w", err)
	}

	if len(script) == 0 {
		return "unknown", nil
	}

	// Simple script type detection
	switch {
	case len(script) == 25 && script[0] == 0x76 && script[1] == 0xa9 && script[2] == 0x14:
		return "p2pkh", nil
	case len(script) == 23 && script[0] == 0xa9 && script[1] == 0x14:
		return "p2sh", nil
	case len(script) == 22 && script[0] == 0x00 && script[1] == 0x14:
		return "p2wpkh", nil
	case len(script) == 34 && script[0] == 0x00 && script[1] == 0x20:
		return "p2wsh", nil
	case len(script) == 34 && script[0] == 0x51 && script[1] == 0x20:
		return "p2tr", nil
	default:
		return "unknown", nil
	}
}

// CalculateConfirmations calculates the number of confirmations for a UTXO
func CalculateConfirmations(utxo *models.UTXO, currentHeight int) int {
	if utxo.BlockHeight == nil {
		return 0 // Mempool transaction
	}

	return currentHeight - *utxo.BlockHeight + 1
}

// IsConfirmed checks if a UTXO is confirmed based on confirmation requirements
func IsConfirmed(utxo *models.UTXO, currentHeight, confirmationsRequired int) bool {
	confirmations := CalculateConfirmations(utxo, currentHeight)
	return confirmations >= confirmationsRequired
}

// ProcessTransactionComprehensive stores complete transaction data
func (p *Parser) ProcessTransactionComprehensive(tx *bitcoin.Tx, blockHeight *int, blockHash *string, blockTime *int64) error {
	// Store transaction
	transaction := &models.Transaction{
		Txid:        tx.Txid,
		BlockHeight: blockHeight,
		BlockHash:   blockHash,
		BlockTime:   p.convertBlockTime(blockTime),
		Size:        &tx.Size,
		Weight:      &tx.Weight,
		FeeSats:     p.calculateFee(tx),
		IsCoinbase:  p.isCoinbase(tx),
		CreatedAt:   time.Now(),
	}

	err := p.db.StoreTransaction(transaction)
	if err != nil {
		return fmt.Errorf("failed to store transaction: %w", err)
	}

	// Store transaction inputs
	for i, vin := range tx.Vin {
		// Store all inputs, including coinbase inputs
		input := &models.TransactionInput{
			Txid:      tx.Txid,
			Vout:      i, // This is the input index in the transaction
			PrevTxid:  vin.Txid,
			PrevVout:  vin.Vout,
			ScriptSig: &vin.ScriptSig.Hex,
			Sequence:  &vin.Sequence,
			Witness:   vin.Txinwitness,
		}

		err := p.db.StoreTransactionInput(input)
		if err != nil {
			return fmt.Errorf("failed to store transaction input: %w", err)
		}
	}

	// Store transaction outputs
	for _, vout := range tx.Vout {
		// Extract address with priority: address field > addresses[0] > derived identifier
		address := ExtractAddressFromScriptPubKey(vout.ScriptPubKey)

		scriptType, _ := GetScriptType(vout.ScriptPubKey.Hex)

		output := &models.TransactionOutput{
			Txid:       tx.Txid,
			Vout:       vout.N,
			Address:    address,
			ValueSats:  int64(vout.Value * 100000000), // Convert BTC to satoshis
			ScriptType: &scriptType,
			ScriptHex:  &vout.ScriptPubKey.Hex,
			ScriptAsm:  &vout.ScriptPubKey.Asm,
		}

		err := p.db.StoreTransactionOutput(output)
		if err != nil {
			return fmt.Errorf("failed to store transaction output: %w", err)
		}
	}

	return nil
}

// ProcessTransactionsBatch processes multiple transactions in a single batch operation
func (p *Parser) ProcessTransactionsBatch(transactions []*bitcoin.Tx, blockHeight *int, blockHash *string, blockTime *int64) error {
	if len(transactions) == 0 {
		return nil
	}

	var txModels []*models.Transaction
	var allInputs []*models.TransactionInput
	var allOutputs []*models.TransactionOutput

	// Prepare all transaction data
	for _, tx := range transactions {
		// Create transaction model
		transaction := &models.Transaction{
			Txid:        tx.Txid,
			BlockHeight: blockHeight,
			BlockHash:   blockHash,
			BlockTime:   p.convertBlockTime(blockTime),
			Size:        &tx.Size,
			Weight:      &tx.Weight,
			FeeSats:     p.calculateFee(tx),
			IsCoinbase:  p.isCoinbase(tx),
			CreatedAt:   time.Now(),
		}
		txModels = append(txModels, transaction)

		// Prepare inputs
		for i, vin := range tx.Vin {
			input := &models.TransactionInput{
				Txid:      tx.Txid,
				Vout:      i,
				PrevTxid:  vin.Txid,
				PrevVout:  vin.Vout,
				ScriptSig: &vin.ScriptSig.Hex,
				Sequence:  &vin.Sequence,
				Witness:   vin.Txinwitness,
			}
			allInputs = append(allInputs, input)
		}

		// Prepare outputs
		for _, vout := range tx.Vout {
			// Extract address with priority: address field > addresses[0] > derived identifier
			address := ExtractAddressFromScriptPubKey(vout.ScriptPubKey)

			scriptType, _ := GetScriptType(vout.ScriptPubKey.Hex)

			output := &models.TransactionOutput{
				Txid:       tx.Txid,
				Vout:       vout.N,
				Address:    address,
				ValueSats:  int64(vout.Value * 100000000),
				ScriptType: &scriptType,
				ScriptHex:  &vout.ScriptPubKey.Hex,
				ScriptAsm:  &vout.ScriptPubKey.Asm,
			}
			allOutputs = append(allOutputs, output)
		}
	}

	// Store all data in a single batch operation
	return p.db.BatchStoreTransactions(txModels, allInputs, allOutputs)
}

// convertBlockTime converts block timestamp to time.Time
func (p *Parser) convertBlockTime(blockTime *int64) *time.Time {
	if blockTime == nil {
		return nil
	}
	t := time.Unix(*blockTime, 0)
	return &t
}

// calculateFee calculates transaction fee
func (p *Parser) calculateFee(tx *bitcoin.Tx) *int64 {
	// Simple fee calculation: sum of inputs - sum of outputs
	// This is a simplified approach; in practice, you'd need to fetch input values
	// For now, return nil to indicate fee calculation is not implemented
	return nil
}

// isCoinbase checks if transaction is coinbase
func (p *Parser) isCoinbase(tx *bitcoin.Tx) bool {
	return len(tx.Vin) == 1 && tx.Vin[0].Txid == "" && tx.Vin[0].Vout == 0
}
