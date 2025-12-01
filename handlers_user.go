package main

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/mdbot/wiki/config"
)

type Authenticator interface {
	Authenticate(username, password string) (*config.User, error)
}

func LoginHandler(auth Authenticator) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			log.Printf("Error parsing form: %v", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		username := request.FormValue("username")
		password := request.FormValue("password")
		redirect := request.FormValue("redirect")

		// Only allow relative redirects
		if !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
			redirect = "/"
		}

		user, err := auth.Authenticate(username, password)
		if err != nil {
			putSessionKey(writer, request, sessionErrorKey, fmt.Sprintf("Failed to login: %v", err))
		} else {
			putSessionKey(writer, request, sessionUserKey, user.Name)
			putSessionKey(writer, request, sessionSessionKey, fmt.Sprintf(sessionKeyFormat, user.SessionKey))
		}
		writer.Header().Set("location", redirect)
		writer.WriteHeader(http.StatusSeeOther)
	}
}

func LogoutHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			log.Printf("Error parsing form: %v", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		redirect := request.FormValue("redirect")

		// Only allow relative redirects
		if !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
			redirect = "/"
		}

		clearSessionKey(writer, request, sessionUserKey)
		writer.Header().Set("location", redirect)
		writer.WriteHeader(http.StatusSeeOther)
	}
}

type UserLister interface {
	Users() []*config.User
}

func ManageUsersHandler(t *Templates, ul UserLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users := ul.Users()
		var infos []UserInfo

		for i := range users {
			infos = append(infos, UserInfo{
				Name:        users[i].Name,
				Permissions: users[i].Permissions.String(),
			})
		}

		sort.Slice(infos, func(i, j int) bool {
			return infos[i].Name < infos[j].Name
		})
		t.RenderManageUsers(w, r, infos)
	}
}

type UserModifier interface {
	AddUser(username, password, responsible string) error
	SetPassword(username, password, responsible string) error
	SetPermission(username string, permissions config.Permission, responsible string) error
	Delete(username, responsible string) error
}

func ModifyUserHandler(um UserModifier) http.HandlerFunc {
	permissionNames := map[string]config.Permission{
		"none":  config.PermissionNone,
		"auth":  config.PermissionAuth,
		"read":  config.PermissionRead,
		"write": config.PermissionWrite,
		"admin": config.PermissionAdmin,
	}

	return func(writer http.ResponseWriter, request *http.Request) {
		responsible := "Anonymoose"
		if user := getUserForRequest(request); user != nil {
			responsible = user.Name
		}

		if err := request.ParseForm(); err != nil {
			log.Printf("Error parsing form: %v", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		user := request.FormValue("user")
		action := request.FormValue("action")
		if action == "password" {
			if err := um.SetPassword(user, request.FormValue("password"), responsible); err != nil {
				putSessionKey(writer, request, sessionErrorKey, fmt.Sprintf("Unable to set password: %v", err))
			} else {
				putSessionKey(writer, request, sessionNoticeKey, fmt.Sprintf("Password updated for user %s", user))
			}
		} else if action == "delete" {
			if err := um.Delete(user, responsible); err != nil {
				putSessionKey(writer, request, sessionErrorKey, fmt.Sprintf("Unable to delete user: %v", err))
			} else {
				putSessionKey(writer, request, sessionNoticeKey, fmt.Sprintf("User %s has been terminated", user))
			}
		} else if action == "new" {
			if err := um.AddUser(user, request.FormValue("password"), responsible); err != nil {
				putSessionKey(writer, request, sessionErrorKey, fmt.Sprintf("Unable to create new user: %v", err))
			} else {
				putSessionKey(writer, request, sessionNoticeKey, fmt.Sprintf("User %s has been created", user))
			}
		} else if action == "permissions" {
			perm, ok := permissionNames[request.FormValue("permissions")]
			if !ok {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			if err := um.SetPermission(user, perm, responsible); err != nil {
				putSessionKey(writer, request, sessionErrorKey, fmt.Sprintf("Unable to set permissions: %v", err))
			} else {
				putSessionKey(writer, request, sessionNoticeKey, fmt.Sprintf("User %s has been modified", user))
			}
		} else {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		writer.Header().Add("location", "/wiki/users")
		writer.WriteHeader(http.StatusSeeOther)
	}
}

func AccountHandler(t *Templates) http.HandlerFunc {
	return t.RenderAccount
}

type PasswordUpdater interface {
	SetPassword(username, password, responsible string) error
	Authenticate(username, password string) (*config.User, error)
}

func ModifyAccountHandler(pu PasswordUpdater) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		user := getUserForRequest(request)
		if user == nil {
			writer.WriteHeader(http.StatusForbidden)
			return
		}

		if err := request.ParseForm(); err != nil {
			log.Printf("Error parsing form: %v", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		action := request.FormValue("action")
		if action == "password" {
			if _, err := pu.Authenticate(user.Name, request.FormValue("password")); err != nil {
				putSessionKey(writer, request, sessionErrorKey, "Your password wa incorrect")
				writer.Header().Add("location", "/wiki/account")
				writer.WriteHeader(http.StatusSeeOther)
				return
			}

			password1 := request.FormValue("password1")
			password2 := request.FormValue("password2")
			if password1 != password2 {
				putSessionKey(writer, request, sessionErrorKey, "New passwords didn't match")
			} else if err := pu.SetPassword(user.Name, password1, user.Name); err != nil {
				putSessionKey(writer, request, sessionErrorKey, fmt.Sprintf("Unable to set password: %v", err))
			} else {
				putSessionKey(writer, request, sessionNoticeKey, "Your password has been updated")
				putSessionKey(writer, request, sessionSessionKey, fmt.Sprintf(sessionKeyFormat, user.SessionKey))
			}
		} else {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		writer.Header().Add("location", "/wiki/account")
		writer.WriteHeader(http.StatusSeeOther)
	}
}
