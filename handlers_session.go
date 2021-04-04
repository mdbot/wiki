package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/mdbot/wiki/config"
)

const (
	sessionName       = "wiki"
	sessionUserKey    = "user"
	sessionSessionKey = "session"
	sessionNoticeKey  = "notice"
	sessionErrorKey   = "error"

	contextUserKey    = "user"
	contextErrorKey   = "error"
	contextNoticeKey  = "notice"
	contextSessionKey = "session"

	sessionKeyFormat = "wiki:%x"
)

type UserProvider interface {
	User(string) *config.User
}

func SessionHandler(up UserProvider, store sessions.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			s, _ := store.Get(request, sessionName)

			if username, ok := s.Values[sessionUserKey]; ok {
				user := up.User(username.(string))
				if user != nil {
					if key := s.Values[sessionSessionKey]; fmt.Sprintf(sessionKeyFormat, user.SessionKey) == key {
						request = request.WithContext(context.WithValue(request.Context(), contextUserKey, user))
					}
				}
			}

			if e, ok := s.Values[sessionErrorKey]; ok {
				request = request.WithContext(context.WithValue(request.Context(), contextErrorKey, e))
			}

			if e, ok := s.Values[sessionNoticeKey]; ok {
				request = request.WithContext(context.WithValue(request.Context(), contextNoticeKey, e))
			}

			request = request.WithContext(context.WithValue(request.Context(), contextSessionKey, s))

			next.ServeHTTP(writer, request)
		})
	}
}

func putSessionKey(w http.ResponseWriter, r *http.Request, key string, value interface{}) {
	if s := getSessionForRequest(r); s != nil {
		s.Values[key] = value

		if s.IsNew {
			s.Options.HttpOnly = true
			s.Options.SameSite = http.SameSiteStrictMode
			s.Options.MaxAge = 60 * 60 * 24 * 31
		}

		if err := s.Save(r, w); err != nil {
			log.Printf("Unable to save session: %v", err)
		}
	}
}

func getUserForRequest(r *http.Request) *config.User {
	v, _ := r.Context().Value(contextUserKey).(*config.User)
	return v
}

func getErrorForRequest(r *http.Request) string {
	v, _ := r.Context().Value(contextErrorKey).(string)
	return v
}

func getNoticeForRequest(r *http.Request) string {
	v, _ := r.Context().Value(contextNoticeKey).(string)
	return v
}

func getSessionForRequest(r *http.Request) *sessions.Session {
	v, _ := r.Context().Value(contextSessionKey).(*sessions.Session)
	return v
}

func clearSessionKey(w http.ResponseWriter, r *http.Request, key string) {
	s := getSessionForRequest(r)
	if s != nil {
		delete(s.Values, key)
		_ = s.Save(r, w)
	}
}
