package gocontroller

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
)

func verifyHS256(signingInput, signature, secret []byte) error {
	mac := hmac.New(sha256.New, secret)
	mac.Write(signingInput)
	expected := mac.Sum(nil)
	if !hmac.Equal(signature, expected) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func verifyRS256(signingInput, signature []byte, pub *rsa.PublicKey) error {
	if pub == nil {
		return fmt.Errorf("public key required for RS256")
	}
	hashed := sha256.Sum256(signingInput)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, hashed[:], signature)
}
