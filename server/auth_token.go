// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

type AuthToken struct {
	Token *oauth2.Token `json:"token,omitempty"`
}

func (p *Plugin) NewEncodedAuthToken(token *oauth2.Token) (encodedToken string, returnErr error) {
	encryptionSecret := config.GetConfig().EncryptionKey

	t := AuthToken{
		Token: token,
	}

	jsonBytes, err := json.Marshal(t)
	if err != nil {
		p.client.Log.Error("Error marshaling the auth token", "error", err.Error())
		return "", err
	}

	encrypted, err := encrypt(jsonBytes, []byte(encryptionSecret))
	if err != nil {
		p.client.Log.Error("Error encrypting the auth token", "error", err.Error())
		return "", err
	}

	return encode(encrypted), nil
}

func (p *Plugin) ParseAuthToken(encoded string) (token *oauth2.Token, returnErr error) {
	t := AuthToken{}
	encryptionSecret := config.GetConfig().EncryptionKey

	decoded, err := decode(encoded)
	if err != nil {
		p.client.Log.Error("Error decoding the auth token", "error", err.Error())
		return nil, err
	}

	jsonBytes, err := decrypt(decoded, []byte(encryptionSecret))
	if err != nil {
		p.client.Log.Error("Error decrypting the auth token", "error", err.Error())
		return nil, err
	}

	if err = json.Unmarshal(jsonBytes, &t); err != nil {
		p.client.Log.Error("Error unmarshaling the auth token", "error", err.Error())
		return nil, err
	}

	return t.Token, nil
}

func encode(encrypted []byte) string {
	encoded := make([]byte, base64.URLEncoding.EncodedLen(len(encrypted)))
	base64.URLEncoding.Encode(encoded, encrypted)
	return string(encoded)
}

func encrypt(plain, secret []byte) ([]byte, error) {
	if len(secret) == 0 {
		return plain, nil
	}

	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	sealed := aesgcm.Seal(nil, nonce, plain, nil)
	return append(nonce, sealed...), nil
}

func decode(encoded string) ([]byte, error) {
	decoded := make([]byte, base64.URLEncoding.DecodedLen(len(encoded)))
	n, err := base64.URLEncoding.Decode(decoded, []byte(encoded))
	if err != nil {
		return nil, err
	}
	return decoded[:n], nil
}

func decrypt(encrypted, secret []byte) ([]byte, error) {
	if len(secret) == 0 {
		return encrypted, nil
	}

	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesgcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, errors.New("token too short")
	}

	nonce, encrypted := encrypted[:nonceSize], encrypted[nonceSize:]
	plain, err := aesgcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, err
	}

	return plain, nil
}
