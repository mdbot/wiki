package config

import (
	"crypto/rand"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const userSettingsName = "users"

type User struct {
	Name     string
	Salt     []byte
	Password []byte
}

type UserManager struct {
	sessionKey []byte
	users      map[string]*User
	store      Store
}

type UserSettings struct {
	Key   []byte
	Users []*User
}

func NewUserManager(store Store) (*UserManager, error) {
	am := &UserManager{
		users: map[string]*User{},
		store: store,
	}

	if err := am.load(); err != nil {
		return nil, err
	}

	if am.sessionKey == nil || len(am.sessionKey) < 32 {
		newKey := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
			return nil, err
		}
		am.sessionKey = newKey
	}

	return am, nil
}

func (a *UserManager) Empty() bool {
	return len(a.users) == 0
}

func (a *UserManager) SessionKey() []byte {
	return a.sessionKey
}

func (a *UserManager) load() error {
	settings := &UserSettings{}
	if err := a.store.GetSettings(userSettingsName, &settings); err != nil {
		return err
	}

	for i := range settings.Users {
		u := settings.Users[i]
		a.users[strings.ToLower(u.Name)] = u
	}

	a.sessionKey = settings.Key
	return nil
}

func (a *UserManager) save(user, message string) error {
	settings := &UserSettings{
		Key: a.sessionKey,
	}

	for i := range a.users {
		settings.Users = append(settings.Users, a.users[i])
	}

	return a.store.PutSettings(userSettingsName, user, message, &settings)
}

func (a *UserManager) Authenticate(username, password string) (*User, error) {
	user, ok := a.users[strings.ToLower(username)]
	if !ok {
		return nil, fmt.Errorf("invalid username/password")
	}

	salted := append([]byte(password), user.Salt...)
	if err := bcrypt.CompareHashAndPassword(user.Password, salted); err != nil {
		return nil, fmt.Errorf("invalid username/password")
	}

	return user, nil
}

func (a *UserManager) User(username string) *User {
	return a.users[strings.ToLower(username)]
}

func (a *UserManager) AddUser(user, newUsername, newPassword string) error {
	if _, ok := a.users[strings.ToLower(newUsername)]; ok {
		return fmt.Errorf("user already exists")
	}

	salt, err := a.generateSalt()
	if err != nil {
		return err
	}

	salted := append([]byte(newPassword), salt...)
	hash, err := bcrypt.GenerateFromPassword(salted, bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	a.users[strings.ToLower(newUsername)] = &User{
		Name:     newUsername,
		Salt:     salt,
		Password: hash,
	}

	return a.save(user, fmt.Sprintf("Adding new user: %s", newUsername))
}

func (a *UserManager) generateSalt() ([]byte, error) {
	res := make([]byte, 16)
	n, err := rand.Read(res)

	if n < 16 || err != nil {
		return nil, fmt.Errorf("unable to generate random bytes. Wanted 16, got: %d, err: %w", n, err)
	}

	return res, nil
}
