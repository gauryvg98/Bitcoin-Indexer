package parser

import (
	"encoding/hex"
	"fmt"

	"bitcoin-indexer/internal/bitcoin"
)

// ExtractAddressFromScript extracts the Bitcoin address from a script hex
func ExtractAddressFromScript(scriptHex string) (string, error) {
	// Use the DeriveAddressFromScript function directly
	address := DeriveAddressFromScript(scriptHex)
	return address, nil
}

// ExtractAddressFromScriptPubKey extracts address with priority:
// 1. Bitcoin Core's "address" field (singular, newer versions)
// 2. Bitcoin Core's "addresses[0]" field (plural, older versions)
// 3. Derived identifier from script hex (fallback for OP_RETURN, etc.)
func ExtractAddressFromScriptPubKey(scriptPubKey bitcoin.ScriptPubKey) string {
	// Priority 1: Check for singular "address" field (newer Bitcoin Core)
	if scriptPubKey.Address != "" {
		return scriptPubKey.Address
	}

	// Priority 2: Check for plural "addresses" field (older Bitcoin Core)
	if len(scriptPubKey.Addresses) > 0 && scriptPubKey.Addresses[0] != "" {
		return scriptPubKey.Addresses[0]
	}

	// Priority 3: Derive identifier from script (for OP_RETURN, unknown types, etc.)
	return DeriveAddressFromScript(scriptPubKey.Hex)
}

// GetScriptTypeFromHex determines script type from hex
func GetScriptTypeFromHex(scriptHex string) string {
	script, err := hex.DecodeString(scriptHex)
	if err != nil {
		return "unknown"
	}

	if len(script) == 0 {
		return "empty"
	}

	// Simple script type detection
	switch {
	case len(script) == 25 && script[0] == 0x76 && script[1] == 0xa9 && script[2] == 0x14:
		return "p2pkh"
	case len(script) == 23 && script[0] == 0xa9 && script[1] == 0x14:
		return "p2sh"
	case len(script) == 22 && script[0] == 0x00 && script[1] == 0x14:
		return "p2wpkh"
	case len(script) == 34 && script[0] == 0x00 && script[1] == 0x20:
		return "p2wsh"
	case len(script) == 34 && script[0] == 0x51 && script[1] == 0x20:
		return "p2tr"
	case script[0] == 0x6a:
		return "op_return"
	default:
		return "unknown"
	}
}

// DeriveAddressFromScript attempts to derive an address from script hex
func DeriveAddressFromScript(scriptHex string) string {
	// Skip the circular dependency and go directly to script analysis

	// If that fails, try to derive from script pattern
	scriptType := GetScriptTypeFromHex(scriptHex)

	switch scriptType {
	case "p2pkh":
		// P2PKH: OP_DUP OP_HASH160 <20-byte-hash> OP_EQUALVERIFY OP_CHECKSIG
		if len(scriptHex) >= 46 { // 76a914 + 40 hex chars + 88ac
			hashHex := scriptHex[6:46] // Extract the 20-byte hash (40 hex chars)
			// Use full hash for better processing
			return fmt.Sprintf("p2pkh_%s", hashHex)
		}
	case "p2sh":
		// P2SH: OP_HASH160 <20-byte-hash> OP_EQUAL
		if len(scriptHex) >= 44 { // a914 + 40 hex chars + 87
			hashHex := scriptHex[4:44] // Extract the 20-byte hash (40 hex chars)
			// Use full hash for better processing
			return fmt.Sprintf("p2sh_%s", hashHex)
		}
	case "p2wpkh":
		// P2WPKH: OP_0 <20-byte-hash>
		if len(scriptHex) >= 44 { // 0014 + 40 hex chars
			hashHex := scriptHex[4:44] // Extract the 20-byte hash (40 hex chars)
			// Use full hash for better processing
			return fmt.Sprintf("p2wpkh_%s", hashHex)
		}
	case "p2wsh":
		// P2WSH: OP_0 <32-byte-hash>
		if len(scriptHex) >= 68 { // 0020 + 64 hex chars
			hashHex := scriptHex[4:68] // Extract the 32-byte hash (64 hex chars)
			// Use full hash for better processing
			return fmt.Sprintf("p2wsh_%s", hashHex)
		}
	case "p2tr":
		// P2TR: OP_1 <32-byte-hash>
		if len(scriptHex) >= 68 { // 5120 + 64 hex chars
			hashHex := scriptHex[4:68] // Extract the 32-byte hash (64 hex chars)
			// Use full hash for better processing
			return fmt.Sprintf("p2tr_%s", hashHex)
		}
	case "op_return":
		// For OP_RETURN, use the data hash for identification
		if len(scriptHex) > 4 { // 6a + data
			dataHex := scriptHex[2:] // Extract the data part
			return fmt.Sprintf("op_return_%s", dataHex[:min(16, len(dataHex))])
		}
		return "null_data"
	}

	// If all else fails, return a hash-based identifier (safely)
	if len(scriptHex) > 16 {
		return fmt.Sprintf("script_%s", scriptHex[:16])
	}
	return fmt.Sprintf("script_%s", scriptHex)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
