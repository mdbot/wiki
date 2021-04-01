package main

import (
	"context"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/sessions"
	"github.com/mdbot/wiki/config"
)

const (
	sessionName     = "wiki"
	sessionUserKey  = "user"
	sessionErrorKey = "error"

	contextUserKey    = "user"
	contextErrorKey   = "error"
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

	return SessionArgs{
		CanEdit:      user != nil,
		Error:        e,
		User:         user,
		CsrfField:    csrf.TemplateField(r),
		RequestedUrl: r.URL.String(),
	}
}

func RequireAnyUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if user := getUserForRequest(request); user != nil {
			next.ServeHTTP(writer, request)
		} else {
			writer.WriteHeader(http.StatusForbidden)
		}
	})
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

func NotFoundHandler(h http.Handler, templateFs fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fakeWriter := &notFoundInterceptWriter{realWriter: w}

		h.ServeHTTP(fakeWriter, r)

		if fakeWriter.status == http.StatusNotFound {
			renderTemplate(templateFs, NotFound, http.StatusNotFound, w, &NotFoundPageArgs{
				CommonPageArgs{
					Session:   getSessionArgs(w, r),
					PageTitle: "Page not found",
				},
			})
		}
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
