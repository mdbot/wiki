package config

import (
	"crypto/rand"
	"errors"
	"fmt"
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
	users      map[string]*User
	store      Store
}

type UserSettings struct {
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

	return am, nil
}

func (a *UserManager) Empty() bool {
	return len(a.users) == 0
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

func (a *UserManager) save(user, message string) error {
	settings := &UserSettings{}

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

func (a *UserManager) Users() []*User {
	var res []*User
	for i := range a.users {
		res = append(res, a.users[i])
	}
	return res
}

func (a *UserManager) Delete(user, responsible string) error {
	if _, ok := a.users[strings.ToLower(user)]; !ok {
		return errors.New("user does not exist")
	}

	if len(a.users) == 1 {
		return errors.New("can't delete last remaining user")
	}

	delete(a.users, strings.ToLower(user))
	return a.save(responsible, fmt.Sprintf("Deleting user: %s", user))
}

func (a *UserManager) SetPassword(user, password, responsible string) error {
	u := a.users[strings.ToLower(user)]
	if u == nil {
		return errors.New("user does not exist")
	}

	if err := a.setPassword(u, password); err != nil {
		return err
	}
	return a.save(responsible, fmt.Sprintf("Changing password for user: %s", user))
}

func (a *UserManager) AddUser(user, password, responsible string) error {
	if _, ok := a.users[strings.ToLower(user)]; ok {
		return fmt.Errorf("user already exists")
	}

	if user == "" {
		return errors.New("invalid username")
	}

	u := &User{
		Name:     user,
	}

	if err := a.setPassword(u, password); err != nil {
		return err
	}

	a.users[strings.ToLower(user)] = u
	return a.save(responsible, fmt.Sprintf("Adding new user: %s", user))
}

func (a *UserManager) generateSalt() ([]byte, error) {
	res := make([]byte, 16)
	n, err := rand.Read(res)

	if n < 16 || err != nil {
		return nil, fmt.Errorf("unable to generate random bytes. Wanted 16, got: %d, err: %w", n, err)
	}

	return res, nil
}

func (a *UserManager) setPassword(u *User, password string) error {
	salt, err := a.generateSalt()
	if err != nil {
		return err
	}

	salted := append([]byte(password), salt...)
	hash, err := bcrypt.GenerateFromPassword(salted, bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	u.Salt = salt
	u.Password = hash
	return nil
}
