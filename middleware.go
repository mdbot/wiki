package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/gorilla/sessions"
)

const (
	sessionName     = "wiki"
	sessionUserKey  = "user"
	sessionErrorKey = "error"
	contextUserKey  = "user"
	contextErrorKey = "error"
)

type UserProvider interface {
	User(string) *User
}

func SessionHandler(up UserProvider, store sessions.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			s, _ := store.Get(request, sessionName)

			if username, ok := s.Values[sessionUserKey]; ok {
				user := up.User(username.(string))
				if user != nil {
					request = request.WithContext(context.WithValue(request.Context(), contextUserKey, user))
				}
			}

			if e, ok := s.Values[sessionErrorKey]; ok {
				request = request.WithContext(context.WithValue(request.Context(), contextErrorKey, e))
			}

			next.ServeHTTP(writer, request)
		})
	}
}

func putSession(store sessions.Store, w http.ResponseWriter, r *http.Request, key string, value interface{}) {
	s, _ := store.Get(r, sessionName)
	s.Values[key] = value

	if s.IsNew {
		s.Options.HttpOnly = true
		s.Options.SameSite = http.SameSiteStrictMode
		s.Options.MaxAge = 60 * 60 * 24 * 31
	}

	if err := store.Save(r, w, s); err != nil {
		log.Printf("Unable to save session: %v", err)
	}

}

func getUserForRequest(r *http.Request) *User {
	v, _ := r.Context().Value(contextUserKey).(*User)
	return v
}

func getErrorForRequest(r *http.Request) string {
	v, _ := r.Context().Value(contextErrorKey).(string)
	return v
}

func getSessionArgs(r *http.Request) SessionArgs {
	user := getUserForRequest(r)
	return SessionArgs{
		CanEdit: user != nil,
		Error:   getErrorForRequest(r),
		User:    user,
	}
}

func NewLoggingHandler(dst io.Writer) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return handlers.LoggingHandler(dst, h)
	}
}

func BasicAuthHandler(realm string, credentials map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok {
				unauthorized(w, realm)
				return
			}
			_, ok = credentials[username]
			if ok && credentials[username] == password {
				next.ServeHTTP(w, r)
				return
			}
			unauthorized(w, realm)
		})
	}
}

func BasicAuthFromEnv() func(http.Handler) http.Handler {
	if *realm == "" {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}
	}
	return BasicAuthHandler(*realm, map[string]string{*username: *password})
}

func unauthorized(w http.ResponseWriter, realm string) {
	w.Header().Add("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
	w.WriteHeader(http.StatusUnauthorized)
}

type notFoundInterceptWriter struct {
	realWriter http.ResponseWriter
	status     int
}

func (w *notFoundInterceptWriter) Header() http.Header {
	return w.realWriter.Header()
}

func (w *notFoundInterceptWriter) WriteHeader(status int) {
	w.status = status
	if status != http.StatusNotFound {
		w.realWriter.WriteHeader(status)
	}
}

func (w *notFoundInterceptWriter) Write(p []byte) (int, error) {
	if w.status != http.StatusNotFound {
		return w.realWriter.Write(p)
	}
	return len(p), nil
}

func NotFoundHandler(h http.Handler, templateFs fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fakeWriter := &notFoundInterceptWriter{realWriter: w}

		h.ServeHTTP(fakeWriter, r)

		if fakeWriter.status == http.StatusNotFound {
			renderTemplate(templateFs, NotFound, http.StatusNotFound, w, &NotFoundPageArgs{
				CommonPageArgs{
					Session:   getSessionArgs(r),
					PageTitle: "Page not found",
				},
			})
		}
	}
}
