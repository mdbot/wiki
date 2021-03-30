package main

import (
	"crypto/rand"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const userSettingsName = "users"

type User struct {
	Name     string `yaml:"name"`
	Salt     []byte `yaml:"salt"`
	Password []byte `yaml:"password"`
}

type SettingsStore interface {
	GetSettings(name string, val interface{}) error
	PutSettings(name string, val interface{}) error
}

type UserManager struct {
	users map[string]*User
	store SettingsStore
}

type UserSettings struct {
	Users []*User `yaml:"users"`
}

func NewUserManager(store SettingsStore) (*UserManager, error) {
	am := &UserManager{
		users: map[string]*User{},
		store: store,
	}
	return am, am.load()
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
	return nil
}

func (a *UserManager) save() error {
	settings := &UserSettings{}
	for i := range a.users {
		settings.Users = append(settings.Users, a.users[i])
	}

	return a.store.PutSettings(userSettingsName, &settings)
}

func (a *UserManager) Authenticate(username, password string) (*User, error) {
	user, ok := a.users[strings.ToLower(username)]
	if !ok {
		return nil, fmt.Errorf("invalid username/password")
	}

	salted := append([]byte(password), user.Salt...)
	if err := bcrypt.CompareHashAndPassword(salted, []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid username/password")
	}

	return user, nil
}

func (a *UserManager) User(username string) *User {
	return a.users[strings.ToLower(username)]
}

func (a *UserManager) AddUser(username, password string) error {
	if _, ok := a.users[strings.ToLower(username)]; ok {
		return fmt.Errorf("user already exists")
	}

	salt, err := a.generateSalt()
	if err != nil {
		return err
	}

	salted := append([]byte(password), salt...)
	hash, err := bcrypt.GenerateFromPassword(salted, bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	a.users[strings.ToLower(username)] = &User{
		Name:     username,
		Salt:     salt,
		Password: hash,
	}

	return a.save()
}

func (a *UserManager) generateSalt() ([]byte, error) {
	res := make([]byte, 16)
	n, err := rand.Read(res)

	if n < 16 || err != nil {
		return nil, fmt.Errorf("unable to generate random bytes. Wanted 16, got: %d, err: %w", n, err)
	}

	return res, nil
}
