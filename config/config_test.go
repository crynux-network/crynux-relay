package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestNormalizePrivateKey(t *testing.T) {
	tests := []struct {
		name       string
		privateKey string
		want       string
	}{
		{
			name:       "without prefix",
			privateKey: "abcdef",
			want:       "abcdef",
		},
		{
			name:       "with lowercase prefix",
			privateKey: "0xabcdef",
			want:       "abcdef",
		},
		{
			name:       "with uppercase prefix",
			privateKey: "  0Xabcdef  ",
			want:       "abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePrivateKey(tt.privateKey); got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestInitConfigNormalizesPrivateKeyFromFile(t *testing.T) {
	t.Cleanup(func() {
		appConfig = nil
	})

	dir := t.TempDir()
	privateKey := "0440cb8b2962699e5ce6835170ba86a085d67477e5581e398674a59feb8e7b9c"
	privateKeyFile := filepath.Join(dir, "private_key")
	jwtKeyFile := filepath.Join(dir, "jwt_key")
	macKeyFile := filepath.Join(dir, "mac_key")

	writeTestFile(t, privateKeyFile, "0x"+privateKey)
	writeTestFile(t, jwtKeyFile, "jwt-secret")
	writeTestFile(t, macKeyFile, "mac-secret")

	content := fmt.Sprintf(`environment: debug
blockchains:
  testnet:
    account:
      address: %q
      private_key_file: %q
http:
  jwt:
    secret_key_file: %q
mac:
  secret_key_file: %q
`, addressFromPrivateKey(t, privateKey), filepath.ToSlash(privateKeyFile), filepath.ToSlash(jwtKeyFile), filepath.ToSlash(macKeyFile))
	writeTestFile(t, filepath.Join(dir, "config.yml"), content)

	if err := InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	blockchain, ok := GetConfig().Blockchains["testnet"]
	if !ok {
		t.Fatal("expected testnet blockchain config")
	}
	if blockchain.Account.PrivateKey != privateKey {
		t.Fatalf("expected normalized private key %s, got %s", privateKey, blockchain.Account.PrivateKey)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func addressFromPrivateKey(t *testing.T, privateKeyHex string) string {
	t.Helper()
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	return crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
}
