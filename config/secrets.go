package config

import (
	"crypto/rand"
	"io"
)

const secretsSettingsName = "secrets"

type Secrets struct {
	SessionKey []byte
}

func LoadSecrets(store Store) (*Secrets, error) {
	s := &Secrets{}
	if err := store.GetSettings(secretsSettingsName, &s); err != nil {
		return nil, err
	}

	dirty := false

	if s.SessionKey == nil || len(s.SessionKey) < 32 {
		newKey := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
			return nil, err
		}
		s.SessionKey = newKey
		dirty = true
	}

	if dirty {
		_ = store.PutSettings(secretsSettingsName, "System", "Initialising secrets", s)
	}

	return s, nil
}
