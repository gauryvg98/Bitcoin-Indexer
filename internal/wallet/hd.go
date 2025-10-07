package wallet

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

// Network parameters
var (
	MainNetParams = chaincfg.MainNetParams
	TestNetParams = chaincfg.TestNet3Params
	RegTestParams = chaincfg.RegressionNetParams
)

// HDWallet handles HD wallet derivation
type HDWallet struct {
	network *chaincfg.Params
}

// NewHDWallet creates a new HD wallet manager
func NewHDWallet(network *chaincfg.Params) *HDWallet {
	return &HDWallet{
		network: network,
	}
}

// DeriveAddresses derives addresses from an xpub for a given derivation scheme
func (hd *HDWallet) DeriveAddresses(xpub string, derivationScheme string, account int, gapLimit int) ([]string, error) {
	// Parse the xpub
	key, err := bip32.B58Deserialize(xpub)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize xpub: %w", err)
	}

	var addresses []string

	switch derivationScheme {
	case "bip84": // P2WPKH (native segwit)
		addresses, err = hd.deriveBIP84Addresses(key, account, gapLimit)
	case "bip49": // P2SH-P2WPKH (wrapped segwit)
		addresses, err = hd.deriveBIP49Addresses(key, account, gapLimit)
	case "bip44": // P2PKH (legacy)
		addresses, err = hd.deriveBIP44Addresses(key, account, gapLimit)
	default:
		return nil, fmt.Errorf("unsupported derivation scheme: %s", derivationScheme)
	}

	return addresses, err
}

// deriveBIP84Addresses derives P2WPKH addresses (native segwit)
func (hd *HDWallet) deriveBIP84Addresses(key *bip32.Key, account int, gapLimit int) ([]string, error) {
	var addresses []string

	// BIP84: m/84'/0'/account'/change/address_index
	// For receive addresses: change = 0
	// For change addresses: change = 1

	// Derive receive addresses
	for i := 0; i < gapLimit; i++ {
		// m/84'/0'/account'/0/i
		path := fmt.Sprintf("m/84'/0'/%d'/0/%d", account, i)
		address, err := hd.deriveAddressAtPath(key, path, "p2wpkh")
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}

	// Derive change addresses
	for i := 0; i < gapLimit; i++ {
		// m/84'/0'/account'/1/i
		path := fmt.Sprintf("m/84'/0'/%d'/1/%d", account, i)
		address, err := hd.deriveAddressAtPath(key, path, "p2wpkh")
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}

	return addresses, nil
}

// deriveBIP49Addresses derives P2SH-P2WPKH addresses (wrapped segwit)
func (hd *HDWallet) deriveBIP49Addresses(key *bip32.Key, account int, gapLimit int) ([]string, error) {
	var addresses []string

	// BIP49: m/49'/0'/account'/change/address_index
	// For receive addresses: change = 0
	// For change addresses: change = 1

	// Derive receive addresses
	for i := 0; i < gapLimit; i++ {
		// m/49'/0'/account'/0/i
		path := fmt.Sprintf("m/49'/0'/%d'/0/%d", account, i)
		address, err := hd.deriveAddressAtPath(key, path, "p2sh-p2wpkh")
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}

	// Derive change addresses
	for i := 0; i < gapLimit; i++ {
		// m/49'/0'/account'/1/i
		path := fmt.Sprintf("m/49'/0'/%d'/1/%d", account, i)
		address, err := hd.deriveAddressAtPath(key, path, "p2sh-p2wpkh")
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}

	return addresses, nil
}

// deriveBIP44Addresses derives P2PKH addresses (legacy)
func (hd *HDWallet) deriveBIP44Addresses(key *bip32.Key, account int, gapLimit int) ([]string, error) {
	var addresses []string

	// BIP44: m/44'/0'/account'/change/address_index
	// For receive addresses: change = 0
	// For change addresses: change = 1

	// Derive receive addresses
	for i := 0; i < gapLimit; i++ {
		// m/44'/0'/account'/0/i
		path := fmt.Sprintf("m/44'/0'/%d'/0/%d", account, i)
		address, err := hd.deriveAddressAtPath(key, path, "p2pkh")
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}

	// Derive change addresses
	for i := 0; i < gapLimit; i++ {
		// m/44'/0'/account'/1/i
		path := fmt.Sprintf("m/44'/0'/%d'/1/%d", account, i)
		address, err := hd.deriveAddressAtPath(key, path, "p2pkh")
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}

	return addresses, nil
}

// deriveKeyFromPath derives a key from a BIP32 derivation path
func (hd *HDWallet) deriveKeyFromPath(key *bip32.Key, path string) (*bip32.Key, error) {
	// Parse the path (simplified implementation)
	// Format: m/84'/0'/account'/change/address_index
	// For now, we'll implement a basic path parsing
	// In a production system, you'd want more robust path parsing
	
	// Remove the 'm/' prefix
	if len(path) < 2 || path[:2] != "m/" {
		return nil, fmt.Errorf("invalid path format: %s", path)
	}
	
	path = path[2:] // Remove 'm/' prefix
	
	// Split by '/' and parse each component
	components := []string{}
	current := ""
	for _, char := range path {
		if char == '/' {
			if current != "" {
				components = append(components, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		components = append(components, current)
	}
	
	// Start with the provided key
	currentKey := key
	
	// Derive through each path component
	for _, component := range components {
		// Parse the component (handle hardened derivation with ')
		hardened := false
		if len(component) > 0 && component[len(component)-1] == '\'' {
			hardened = true
			component = component[:len(component)-1]
		}
		
		// Convert to uint32
		var index uint32
		if _, err := fmt.Sscanf(component, "%d", &index); err != nil {
			return nil, fmt.Errorf("invalid path component: %s", component)
		}
		
		// Add hardened flag
		if hardened {
			index += 0x80000000
		}
		
		// Derive the child key
		childKey, err := currentKey.NewChildKey(index)
		if err != nil {
			return nil, fmt.Errorf("failed to derive child key at index %d: %w", index, err)
		}
		
		currentKey = childKey
	}
	
	return currentKey, nil
}

// deriveAddressAtPath derives an address at a specific derivation path
func (hd *HDWallet) deriveAddressAtPath(key *bip32.Key, path, addressType string) (string, error) {
	// Parse the derivation path and derive the key
	derivedKey, err := hd.deriveKeyFromPath(key, path)
	if err != nil {
		return "", fmt.Errorf("failed to derive key from path %s: %w", path, err)
	}

	// Get the public key bytes
	pubKeyBytes := derivedKey.PublicKey().Key

	// Create address based on type
	switch addressType {
	case "p2wpkh": // Native segwit (bech32)
		address, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(pubKeyBytes), hd.network)
		if err != nil {
			return "", err
		}
		return address.EncodeAddress(), nil

	case "p2sh-p2wpkh": // Wrapped segwit (P2SH)
		witnessProg := btcutil.Hash160(pubKeyBytes)
		address, err := btcutil.NewAddressScriptHash(witnessProg, hd.network)
		if err != nil {
			return "", err
		}
		return address.EncodeAddress(), nil

	case "p2pkh": // Legacy
		address, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pubKeyBytes), hd.network)
		if err != nil {
			return "", err
		}
		return address.EncodeAddress(), nil

	default:
		return "", fmt.Errorf("unsupported address type: %s", addressType)
	}
}

// GetScriptHex returns the script hex for an address
func (hd *HDWallet) GetScriptHex(address string) (string, error) {
	// Decode the address
	addr, err := btcutil.DecodeAddress(address, hd.network)
	if err != nil {
		return "", fmt.Errorf("failed to decode address: %w", err)
	}

	// Get the script - using a simplified approach
	// In a real implementation, you'd use the proper script generation
	script := []byte{0x00, 0x14} // Placeholder P2WPKH script
	script = append(script, addr.ScriptAddress()...)
	
	return hex.EncodeToString(script), nil
}

// GetAddressType returns the type of address (p2wpkh, p2sh-p2wpkh, p2pkh, etc.)
func (hd *HDWallet) GetAddressType(address string) (string, error) {
	// Decode the address
	addr, err := btcutil.DecodeAddress(address, hd.network)
	if err != nil {
		return "", fmt.Errorf("failed to decode address: %w", err)
	}

	// Determine address type
	switch addr.(type) {
	case *btcutil.AddressWitnessPubKeyHash:
		return "p2wpkh", nil
	case *btcutil.AddressScriptHash:
		return "p2sh", nil
	case *btcutil.AddressPubKeyHash:
		return "p2pkh", nil
	case *btcutil.AddressWitnessScriptHash:
		return "p2wsh", nil
	default:
		return "unknown", nil
	}
}

// ValidateAddress validates a Bitcoin address
func (hd *HDWallet) ValidateAddress(address string) bool {
	_, err := btcutil.DecodeAddress(address, hd.network)
	return err == nil
}

// GenerateMnemonic generates a new mnemonic phrase
func GenerateMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}
	return bip39.NewMnemonic(entropy)
}

// MnemonicToSeed converts a mnemonic to a seed
func MnemonicToSeed(mnemonic, passphrase string) ([]byte, error) {
	return bip39.NewSeed(mnemonic, passphrase), nil
}

// SeedToMasterKey creates a master key from a seed
func SeedToMasterKey(seed []byte) (*bip32.Key, error) {
	return bip32.NewMasterKey(seed)
}

// MasterKeyToXpub converts a master key to an xpub
func MasterKeyToXpub(masterKey *bip32.Key) string {
	return masterKey.B58Serialize()
}
