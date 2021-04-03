package main

import (
	"net/http"
	"strings"
)

type HistoryProvider interface {
	PageHistory(path string, start string, end int) (*History, error)
}

func PageHistoryHandler(t *Templates, pp HistoryProvider) http.HandlerFunc {
	const historySize = 20

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

		var entries []*HistoryEntry
		for i := range history.Entries {
			c := history.Entries[i]
			if c.ChangeId == start {
				continue
			}
			entries = append(entries, &HistoryEntry{
				Id:      c.ChangeId,
				User:    c.User,
				Time:    c.Time,
				Message: c.Message,
			})
			if len(entries) == historySize {
				break
			}
		}

		var next string
		if len(history.Entries) == number {
			next = history.Entries[number-2].ChangeId
		}

		t.RenderHistory(w, r, pageTitle, entries, next)
	}
}
