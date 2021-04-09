package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type HistoryProvider interface {
	PageHistory(path string, start string, end int) (*History, error)
}

func PageHistoryHandler(t *Templates, pp HistoryProvider) http.HandlerFunc {
	const historySize = 50

	return func(w http.ResponseWriter, r *http.Request) {
		pageTitle := strings.TrimPrefix(r.URL.Path, "/history/")

		var start string
		var number = historySize + 1

		q := r.URL.Query()["after"]
		if q != nil {
			// If the user is paginating, request 22 items so we get the start item, the 20 we want to show, then
			// an extra one to tell if there's a next page or not.
			start = q[0]
			number = historySize + 2
		}

		history, err := pp.PageHistory(pageTitle, start, number)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var next string
		if len(history.Entries) == number {
			next = history.Entries[number-1].ChangeId
		} else {
			number = len(history.Entries) + 1
		}

		t.RenderHistory(w, r, pageTitle, history.Entries[:number-1], next)
	}
}

type RecentChangesProvider interface {
	RecentChanges(start string, count int) ([]*RecentChange, error)
}

func RecentChangesHandler(t *Templates, rp RecentChangesProvider) http.HandlerFunc {
	const historySize = 50

	return func(w http.ResponseWriter, r *http.Request) {
		var start string
		var number = historySize + 1

		q := r.URL.Query()["after"]
		if q != nil {
			// If the user is paginating, request 22 items so we get the start item, the 20 we want to show, then
			// an extra one to tell if there's a next page or not.
			start = q[0]
			number = historySize + 2
		}

		history, err := rp.RecentChanges(start, number)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var next string
		if len(history) == number {
			next = history[number-1].ChangeId
		} else {
			number = len(history) + 1
		}

		t.RenderRecentChanges(w, r, history[:number-1], next)
	}
}

func RecentChangesFeed(t *Templates, rp RecentChangesProvider) http.HandlerFunc {
	const historySize = 50

	return func(w http.ResponseWriter, r *http.Request) {
		history, err := rp.RecentChanges("", historySize)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		t.RenderRecentChangesFeed(w, r, history)
	}
}

type DiffProvider interface {
	PathDiff(path string, startRevision string, endRevision string) ([]diffmatchpatch.Diff, error)
}

func DiffPageHandler(templates *Templates, backend DiffProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageTitle := strings.TrimPrefix(r.URL.Path, "/diff/")
		startRevision := r.FormValue("startrev")
		endRevision := r.FormValue("endrev")
		if startRevision == "" || endRevision == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		diff, err := backend.PathDiff(pageTitle, startRevision, endRevision)
		if err != nil {
			log.Printf("Error getting diff: %+s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		templates.RenderDiff(w, r, diff)
	}
}
