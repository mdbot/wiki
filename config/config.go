package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
)

type Backend interface {
	GetConfig(name string) ([]byte, error)
	PutConfig(name string, content []byte, user, message string) error
}

type Store interface {
	GetSettings(name string, val interface{}) error
	PutSettings(name, user, message string, val interface{}) error
}

func NewStore(backend Backend, key string) Store {
	keyBytes, _ := hex.DecodeString(key)
	if len(keyBytes) == 32 {
		var k [32]byte
		copy(k[:], keyBytes)
		return &EncryptedStore{
			key:     k,
			backend: backend,
		}
	} else {
		return &DummyStore{}
	}
}

type EncryptedStore struct {
	key     [32]byte
	backend Backend
}

func (c EncryptedStore) GetSettings(name string, val interface{}) error {
	data, err := c.backend.GetConfig(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	if len(data) < gcm.NonceSize() {
		return errors.New("malformed ciphertext")
	}

	decrypted, err := gcm.Open(nil,
		data[:gcm.NonceSize()],
		data[gcm.NonceSize():],
		nil,
	)
	if err != nil {
		return err
	}

	return json.Unmarshal(decrypted, &val)
}

func (c EncryptedStore) PutSettings(name, user, message string, val interface{}) error {
	data, err := json.Marshal(&val)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return err
	}

	encrypted := gcm.Seal(nonce, nonce, data, nil)
	return c.backend.PutConfig(name, encrypted, user, message)
}

type DummyStore struct{}

func (d DummyStore) GetSettings(name string, _ interface{}) error {
	log.Printf("Warning: no encryption key specified, using a blank '%s' config\n", name)
	return nil
}

func (d DummyStore) PutSettings(string, string, string, interface{}) error {
	return errors.New("no encryption key specified; config cannot be saved")
}
