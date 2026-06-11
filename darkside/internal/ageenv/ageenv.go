// Package ageenv wraps filippo.io/age for darkside's per-environment encrypted
// env files. Keep the surface narrow: generate keypairs, encrypt, decrypt,
// parse k=v env files.
package ageenv

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
)

// Keypair is what CreateEnvironment hands back to the user.
type Keypair struct {
	PublicKey  string // "age1..." — committed to the repo as the recipient
	PrivateKey string // "AGE-SECRET-KEY-1..." — kept locally; we store it in DB too
}

// GenerateKeypair makes a fresh X25519 age identity.
func GenerateKeypair() (Keypair, error) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return Keypair{}, fmt.Errorf("generate age identity: %w", err)
	}
	return Keypair{
		PublicKey:  id.Recipient().String(),
		PrivateKey: id.String(),
	}, nil
}

// Decrypt reads age-encrypted ciphertext and returns the plaintext.
func Decrypt(ciphertext []byte, privateKey string) ([]byte, error) {
	id, err := age.ParseX25519Identity(privateKey)
	if err != nil {
		return nil, fmt.Errorf("parse age private key: %w", err)
	}
	r, err := age.Decrypt(bytes.NewReader(ciphertext), id)
	if err != nil {
		return nil, fmt.Errorf("age decrypt: %w", err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read decrypted: %w", err)
	}
	return plain, nil
}

// ParseEnvFile takes plaintext env contents (key=value per line, # comments)
// and returns a map. The format intentionally matches the docker --env-file
// format so users can copy-paste between local dev and darkside.
func ParseEnvFile(plain []byte) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(plain))
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("env file line %d: expected KEY=VALUE, got %q", lineNo, line)
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		// Strip optional surrounding quotes.
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[0] == val[len(val)-1] {
			val = val[1 : len(val)-1]
		}
		out[key] = val
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan env file: %w", err)
	}
	return out, nil
}
