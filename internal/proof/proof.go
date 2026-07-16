package proof

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

type Device struct {
	DeviceID   string
	PrivateKey string
	PublicKey  string
}

type Value struct {
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
	Timestamp int64  `json:"timestamp"`
}

func NewDevice() (Device, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Device{}, err
	}
	deviceID, err := randomUUID()
	if err != nil {
		return Device{}, err
	}

	return Device{
		DeviceID:   deviceID,
		PrivateKey: base64.RawURLEncoding.EncodeToString(privateKey),
		PublicKey:  base64.RawURLEncoding.EncodeToString(publicKey),
	}, nil
}

func RandomVerifier() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func Sign(privateKeyValue, purpose, subject string, now time.Time) (Value, error) {
	privateKey, err := base64.RawURLEncoding.DecodeString(privateKeyValue)
	if err != nil {
		return Value{}, err
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return Value{}, fmt.Errorf("invalid Ed25519 private key")
	}
	nonce, err := RandomVerifier()
	if err != nil {
		return Value{}, err
	}
	timestamp := now.Unix()
	message := fmt.Sprintf(
		"unui-cli-proof-v1\n%s\n%s\n%d\n%s",
		purpose,
		subject,
		timestamp,
		nonce,
	)
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), []byte(message))
	return Value{
		Nonce:     nonce,
		Signature: base64.RawURLEncoding.EncodeToString(signature),
		Timestamp: timestamp,
	}, nil
}

func randomUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value)
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		encoded[0:8],
		encoded[8:12],
		encoded[12:16],
		encoded[16:20],
		encoded[20:32],
	), nil
}
