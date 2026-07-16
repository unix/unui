package proof

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestSignMatchesProtocolMessage(t *testing.T) {
	device, err := NewDevice()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	value, err := Sign(device.PrivateKey, "access", device.DeviceID, now)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(device.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(value.Signature)
	if err != nil {
		t.Fatal(err)
	}
	message := strings.Join([]string{
		"unui-cli-proof-v1",
		"access",
		device.DeviceID,
		"1700000000",
		value.Nonce,
	}, "\n")
	if !ed25519.Verify(ed25519.PublicKey(publicKey), []byte(message), signature) {
		t.Fatal("signature did not verify")
	}
}

func TestChallengeIsBase64URLSHA256(t *testing.T) {
	if got := Challenge("test-verifier"); got != "JBbiqONGWPaAmwXk_8bT6UnlPfrn65D32eZlJS-zGG0" {
		t.Fatalf("unexpected challenge: %s", got)
	}
}
