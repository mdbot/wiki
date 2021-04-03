package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/mdbot/wiki/config"
)

func CheckAuthentication(authForReads bool, authForWrites bool) func(http.Handler) http.Handler {
	authRequirements := map[string]bool{
		"/edit/":       authForWrites,
		"/file/":       authForReads,
		"/history/":    authForReads,
		"/view/":       authForReads,
		"/rename/":		authForWrites,
		"/delete/":		authForWrites,
		"/wiki/index":  authForReads,
		"/wiki/files":  authForReads,
		"/wiki/login":  false,
		"/wiki/logout": false,
		"/wiki/upload": authForWrites,
		"/wiki/users":  authForWrites,
	}

	findPrefix := func(target string) (bool, error) {
		for i := range authRequirements {
			if strings.HasPrefix(target, i) {
				return authRequirements[i], nil
			}
		}
		return false, fmt.Errorf("unknown route: %s", target)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			needAuth, err := findPrefix(request.URL.Path)
			if err != nil {
				writer.WriteHeader(http.StatusNotFound)
				return
			}

			if needAuth {
				if getUserForRequest(request) == nil {
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
			}

			next.ServeHTTP(writer, request)
		})
	}
}

type Authenticator interface {
	Authenticate(username, password string) (*config.User, error)
}

func LoginHandler(auth Authenticator) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
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
		}
		writer.Header().Set("location", redirect)
		writer.WriteHeader(http.StatusSeeOther)
	}
}

func LogoutHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
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

type ManageUsersArgs struct {
	CommonPageArgs
	Users []string
}

type UserLister interface {
	Users() []*config.User
}

func ManageUsersHandler(templateFs fs.FS, ul UserLister) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		users := ul.Users()
		var usernames []string

		for i := range users {
			usernames = append(usernames, users[i].Name)
		}

		renderTemplate(templateFs, ManageUsersPage, http.StatusOK, writer, &ManageUsersArgs{
			CommonPageArgs: CommonPageArgs{
				Session:   getSessionArgs(writer, request),
				PageTitle: "Manage users",
			},
			Users: usernames,
		})
	}
}

type UserModifier interface {
	AddUser(username, password, responsible string) error
	SetPassword(username, password, responsible string) error
	Delete(username, responsible string) error
}

func ModifyUserHandler(um UserModifier) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		responsible := "Anonymoose"
		if user := getUserForRequest(request); user != nil {
			responsible = user.Name
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
		} else {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		writer.Header().Add("location", "/wiki/users")
		writer.WriteHeader(http.StatusSeeOther)
	}
}
