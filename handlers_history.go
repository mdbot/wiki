package main

import (
	"io/fs"
	"net/http"
	"strings"
	"time"
)

type HistoryProvider interface {
	PageHistory(path string, start string, end int) (*History, error)
}

type HistoryEntry struct {
	Id      string
	User    string
	Time    time.Time
	Message string
}

type HistoryPageArgs struct {
	CommonPageArgs
	History []*HistoryEntry
	Next    string
}

func PageHistoryHandler(templateFs fs.FS, pp HistoryProvider) http.HandlerFunc {
	const historySize = 20

	return func(writer http.ResponseWriter, request *http.Request) {
		pageTitle := strings.TrimPrefix(request.URL.Path, "/history/")

		var start string
		var number = historySize + 1

		q := request.URL.Query()["after"]
		if q != nil {
			// If the user is paginating, request 22 items so we get the start item, the 20 we want to show, then
			// an extra one to tell if there's a next page or not.
			start = q[0]
			number = historySize + 2
		}

		history, err := pp.PageHistory(pageTitle, start, number)
		if err != nil {
			http.NotFound(writer, request)
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

		renderTemplate(templateFs, HistoryPage, http.StatusOK, writer, &HistoryPageArgs{
			CommonPageArgs: CommonPageArgs{
				Session:    getSessionArgs(writer, request),
				PageTitle:  pageTitle,
				IsWikiPage: true,
			},

			History: entries,
			Next:    next,
		})
	}
}
