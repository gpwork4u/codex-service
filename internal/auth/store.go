package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	AccountID    string    `json:"account_id"`
}

type Store struct {
	dir string
}

func NewStore(dataDir string) *Store {
	return &Store{dir: dataDir}
}

func (s *Store) path() string {
	return filepath.Join(s.dir, "credentials.json")
}

func (s *Store) Load() (*Credentials, error) {
	data, err := os.ReadFile(s.path())
	if err != nil {
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

func (s *Store) Save(creds *Credentials) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), data, 0600)
}
