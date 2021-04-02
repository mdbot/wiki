package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/mdbot/wiki/config"
)

const (
	sessionName      = "wiki"
	sessionUserKey   = "user"
	sessionNoticeKey = "notice"
	sessionErrorKey  = "error"

	contextUserKey    = "user"
	contextErrorKey   = "error"
	contextNoticeKey  = "notice"
	contextSessionKey = "session"
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
					request = request.WithContext(context.WithValue(request.Context(), contextUserKey, user))
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

func getSessionArgs(w http.ResponseWriter, r *http.Request) SessionArgs {
	user := getUserForRequest(r)

	e := getErrorForRequest(r)
	if e != "" {
		clearSessionKey(w, r, sessionErrorKey)
	}

	notice := getNoticeForRequest(r)
	if notice != "" {
		clearSessionKey(w, r, sessionNoticeKey)
	}

	return SessionArgs{
		CanEdit:      user != nil,
		Error:        e,
		Notice: notice,
		User:         user,
		CsrfField:    csrf.TemplateField(r),
		RequestedUrl: r.URL.String(),
	}
}

func CheckAuthentication(authForReads bool, authForWrites bool) mux.MiddlewareFunc {
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

func NewLoggingHandler(dst io.Writer) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return handlers.LoggingHandler(dst, h)
	}
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

func PageErrorHandler(templateFs fs.FS) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fakeWriter := &notFoundInterceptWriter{realWriter: w}

			next.ServeHTTP(fakeWriter, r)

			if fakeWriter.status == http.StatusNotFound {
				isWiki := false
				if strings.HasPrefix(r.RequestURI, "/view/") || strings.HasPrefix(r.RequestURI, "/history") {
					isWiki = true
				}

				renderTemplate(templateFs, NotFound, http.StatusNotFound, w, &ErrorPageArgs{
					CommonPageArgs{
						Session:      getSessionArgs(w, r),
						PageTitle:    "Page not found",
						IsWikiPage:   isWiki,
						IsError: true,
					},
					false,
				})
			}
			if fakeWriter.status == http.StatusUnauthorized {
				renderTemplate(templateFs, Unauthorized, http.StatusUnauthorized, w, &ErrorPageArgs{
					CommonPageArgs{
						Session:   getSessionArgs(w, r),
						PageTitle: "Unauthorized",
						IsError: true,
					},
					true,
				})
			}
			if fakeWriter.status == http.StatusForbidden {
				renderTemplate(templateFs, Forbidden, http.StatusForbidden, w, &ErrorPageArgs{
					CommonPageArgs{
						Session:   getSessionArgs(w, r),
						PageTitle: "Forbidden",
						IsError: true,
					},
					false,
				})
			}
			if fakeWriter.status == http.StatusInternalServerError {
				renderTemplate(templateFs, ServerError, http.StatusInternalServerError, w, &ErrorPageArgs{
					CommonPageArgs{
						Session:   getSessionArgs(w, r),
						PageTitle: "Server Error",
						IsError: true,
					},
					false,
				})
			}
		})
	}
}

func LowerCaseCanonical(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.ToLower(request.RequestURI) != request.RequestURI {
			http.Redirect(writer, request, strings.ToLower(request.RequestURI), http.StatusPermanentRedirect)
		} else {
			next.ServeHTTP(writer, request)
		}
	})
}
