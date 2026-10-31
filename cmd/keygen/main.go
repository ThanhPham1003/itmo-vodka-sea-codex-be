// Command keygen explicitly provisions a new PASETO key pair for local setup.
// It is never called by the API process. Redirect stdout to a protected file.
package main

import (
	"fmt"
	"os"

	paseto "aidanwoods.dev/go-paseto"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/keygen > .env.keys")
		os.Exit(1)
	}
	key := paseto.NewV4AsymmetricSecretKey()
	if _, err := fmt.Fprintf(os.Stdout, "AUTH_PRIVATE_KEY_HEX=%s\nAUTH_PUBLIC_KEY_HEX=%s\n", key.ExportHex(), key.Public().ExportHex()); err != nil {
		fmt.Fprintln(os.Stderr, "cannot write generated key pair")
		os.Exit(1)
	}
}
