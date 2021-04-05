package main

import (
	"net/http"
	"strings"
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
			http.NotFound(w, r)
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
			http.NotFound(w, r)
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