package funcs

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
	"zombiezen.com/go/sqlite"
)

// HashFuncs returns hashing and encoding functions.
func HashFuncs() []Func {
	return []Func{
		{
			Name:          "hash_password",
			NumArgs:       1,
			Deterministic: false, // bcrypt generates random salt each time
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				password := args[0].Text()
				// Cost 12 is a good balance between security and performance
				hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
				if err != nil {
					// Return NULL on error to prevent empty password hashes
					return sqlite.Value{}, err
				}
				return sqlite.TextValue(string(hash)), nil
			},
		},
		{
			Name:          "verify_password",
			NumArgs:       2,
			Deterministic: false, // Result depends on stored hash which varies per user
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				password := args[0].Text()
				storedHash := args[1].Text()
				err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
				if err != nil {
					return sqlite.IntegerValue(0), nil // Password doesn't match
				}
				return sqlite.IntegerValue(1), nil // Password matches
			},
		},
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
