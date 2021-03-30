package main

import "time"

type LogEntry struct {
	ChangeId string
	User     string
	Time     time.Time
	Message  string
}

type Page struct {
	Content      []byte
	LastModified *LogEntry
}

type History struct {
	Entries []*LogEntry
}
