package funcs

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"zombiezen.com/go/sqlite"
)

// HashFuncs returns hashing and encoding functions.
func HashFuncs() []Func {
	return []Func{
		{
			Name:          "hash_md5",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				data := args[0].Text()
				sum := md5.Sum([]byte(data))
				return sqlite.TextValue(hex.EncodeToString(sum[:])), nil
			},
		},
		{
			Name:          "hash_sha256",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				data := args[0].Text()
				sum := sha256.Sum256([]byte(data))
				return sqlite.TextValue(hex.EncodeToString(sum[:])), nil
			},
		},
		{
			Name:          "base64_encode",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				data := args[0].Text()
				encoded := base64.StdEncoding.EncodeToString([]byte(data))
				return sqlite.TextValue(encoded), nil
			},
		},
		{
			Name:          "base64_decode",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				data := args[0].Text()
				decoded, err := base64.StdEncoding.DecodeString(data)
				if err != nil {
					return sqlite.TextValue(""), nil
				}
				return sqlite.TextValue(string(decoded)), nil
			},
		},
		{
			Name:          "hex_encode",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				data := args[0].Text()
				return sqlite.TextValue(hex.EncodeToString([]byte(data))), nil
			},
		},
		{
			Name:          "hex_decode",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				data := args[0].Text()
				decoded, err := hex.DecodeString(data)
				if err != nil {
					return sqlite.TextValue(""), nil
				}
				return sqlite.TextValue(string(decoded)), nil
			},
		},
	}
}
