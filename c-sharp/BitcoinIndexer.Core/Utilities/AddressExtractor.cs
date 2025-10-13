using System;
using System.Linq;

namespace BitcoinIndexer.Core.Utilities;

/// <summary>
/// Utility class for extracting Bitcoin addresses from script data
/// </summary>
public static class AddressExtractor
{
    /// <summary>
    /// Extracts address with priority:
    /// 1. Bitcoin Core's "address" field (singular, newer versions)
    /// 2. Bitcoin Core's "addresses[0]" field (plural, older versions)
    /// 3. Derived identifier from script hex (fallback for OP_RETURN, etc.)
    /// </summary>
    /// <param name="scriptPubKey">The script public key data</param>
    /// <returns>Extracted address or derived identifier</returns>
    public static string ExtractAddressFromScriptPubKey(dynamic scriptPubKey)
    {
        // Priority 1: Check for singular "address" field (newer Bitcoin Core)
        if (!string.IsNullOrEmpty(scriptPubKey.Address))
        {
            return scriptPubKey.Address;
        }

        // Priority 2: Check for plural "addresses" field (older Bitcoin Core)
        if (scriptPubKey.Addresses != null && scriptPubKey.Addresses.Length > 0 && 
            !string.IsNullOrEmpty(scriptPubKey.Addresses[0]))
        {
            return scriptPubKey.Addresses[0];
        }

        // Priority 3: Derive identifier from script (for OP_RETURN, unknown types, etc.)
        return DeriveAddressFromScript(scriptPubKey.Hex);
    }

    /// <summary>
    /// Determines script type from hex
    /// </summary>
    /// <param name="scriptHex">Script in hexadecimal format</param>
    /// <returns>Script type string</returns>
    public static string GetScriptTypeFromHex(string scriptHex)
    {
        if (string.IsNullOrEmpty(scriptHex))
        {
            return "empty";
        }

        try
        {
            var script = Convert.FromHexString(scriptHex);
            
            if (script.Length == 0)
            {
                return "empty";
            }

            // Simple script type detection
            if (script.Length == 25 && script[0] == 0x76 && script[1] == 0xa9 && script[2] == 0x14)
            {
                return "p2pkh";
            }
            else if (script.Length == 23 && script[0] == 0xa9 && script[1] == 0x14)
            {
                return "p2sh";
            }
            else if (script.Length == 22 && script[0] == 0x00 && script[1] == 0x14)
            {
                return "p2wpkh";
            }
            else if (script.Length == 34 && script[0] == 0x00 && script[1] == 0x20)
            {
                return "p2wsh";
            }
            else if (script.Length == 34 && script[0] == 0x51 && script[1] == 0x20)
            {
                return "p2tr";
            }
            else if (script[0] == 0x6a)
            {
                return "op_return";
            }
            else
            {
                return "unknown";
            }
        }
        catch
        {
            return "unknown";
        }
    }

    /// <summary>
    /// Attempts to derive an address from script hex
    /// </summary>
    /// <param name="scriptHex">Script in hexadecimal format</param>
    /// <returns>Derived address identifier</returns>
    public static string DeriveAddressFromScript(string scriptHex)
    {
        if (string.IsNullOrEmpty(scriptHex))
        {
            return "empty_script";
        }

        var scriptType = GetScriptTypeFromHex(scriptHex);

        switch (scriptType)
        {
            case "p2pkh":
                // P2PKH: OP_DUP OP_HASH160 <20-byte-hash> OP_EQUALVERIFY OP_CHECKSIG
                if (scriptHex.Length >= 46) // 76a914 + 40 hex chars + 88ac
                {
                    var hashHex = scriptHex.Substring(6, 40); // Extract the 20-byte hash (40 hex chars)
                    return $"p2pkh_{hashHex}";
                }
                break;

            case "p2sh":
                // P2SH: OP_HASH160 <20-byte-hash> OP_EQUAL
                if (scriptHex.Length >= 44) // a914 + 40 hex chars + 87
                {
                    var hashHex = scriptHex.Substring(4, 40); // Extract the 20-byte hash (40 hex chars)
                    return $"p2sh_{hashHex}";
                }
                break;

            case "p2wpkh":
                // P2WPKH: OP_0 <20-byte-hash>
                if (scriptHex.Length >= 44) // 0014 + 40 hex chars
                {
                    var hashHex = scriptHex.Substring(4, 40); // Extract the 20-byte hash (40 hex chars)
                    return $"p2wpkh_{hashHex}";
                }
                break;

            case "p2wsh":
                // P2WSH: OP_0 <32-byte-hash>
                if (scriptHex.Length >= 68) // 0020 + 64 hex chars
                {
                    var hashHex = scriptHex.Substring(4, 64); // Extract the 32-byte hash (64 hex chars)
                    return $"p2wsh_{hashHex}";
                }
                break;

            case "p2tr":
                // P2TR: OP_1 <32-byte-hash>
                if (scriptHex.Length >= 68) // 5120 + 64 hex chars
                {
                    var hashHex = scriptHex.Substring(4, 64); // Extract the 32-byte hash (64 hex chars)
                    return $"p2tr_{hashHex}";
                }
                break;

            case "op_return":
                // For OP_RETURN, use the data hash for identification
                if (scriptHex.Length > 4) // 6a + data
                {
                    var dataHex = scriptHex.Substring(2); // Extract the data part
                    var truncatedData = dataHex.Length > 16 ? dataHex.Substring(0, 16) : dataHex;
                    return $"op_return_{truncatedData}";
                }
                return "null_data";

            default:
                // If all else fails, return a hash-based identifier (safely)
                if (scriptHex.Length > 16)
                {
                    return $"script_{scriptHex.Substring(0, 16)}";
                }
                return $"script_{scriptHex}";
        }

        return "unknown_script";
    }
}
