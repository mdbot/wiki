package main

import (
	"log"
	"net/http"

	"github.com/mdbot/wiki/config"
)

type PermissionChecker struct {
	requireAuthForWrites bool
	requireAuthForReads  bool
}

func (p *PermissionChecker) CanRead(user *config.User) bool {
	if p.requireAuthForReads {
		return user != nil && user.Has(config.PermissionRead)
	} else {
		return true
	}
}

func (p *PermissionChecker) CanWrite(user *config.User) bool {
	if p.requireAuthForWrites {
		return user != nil && user.Has(config.PermissionWrite)
	} else {
		return true
	}
}

func (p *PermissionChecker) CanAdmin(user *config.User) bool {
	return user != nil && user.Has(config.PermissionAdmin)
}

func (p *PermissionChecker) RequireRead(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := getUserForRequest(r)
		if p.CanRead(user) {
			next.ServeHTTP(w, r)
		} else if user == nil {
			log.Printf("Anonymous user tried to access read-protected resource %s", r.URL)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			log.Printf("User %s (permissions: %s) tried to access read-protected resource %s", user.Name, user.Permissions.String(), r.URL)
			w.WriteHeader(http.StatusForbidden)
		}
	})
}

func (p *PermissionChecker) RequireWrite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := getUserForRequest(r)
		if p.CanWrite(user) {
			next.ServeHTTP(w, r)
		} else if user == nil {
			log.Printf("Anonymous user tried to access write-protected resource %s", r.URL)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			log.Printf("User %s (permissions: %s) tried to access write-protected resource %s", user.Name, user.Permissions.String(), r.URL)
			w.WriteHeader(http.StatusForbidden)
		}
	})
}

func (p *PermissionChecker) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := getUserForRequest(r)
		if p.CanAdmin(user) {
			next.ServeHTTP(w, r)
		} else if user == nil {
			log.Printf("Anonymous user tried to access admin-protected resource %s", r.URL)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			log.Printf("User %s (permissions: %s) tried to access admin-protected resource %s", user.Name, user.Permissions.String(), r.URL)
			w.WriteHeader(http.StatusForbidden)
		}
	})
}

func (p *PermissionChecker) RequireAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := getUserForRequest(r)
		if user == nil {
			log.Printf("Anonymous user tried to access account-protected resource %s", r.URL)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
