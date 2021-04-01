module github.com/greboid/wiki

go 1.16

require (
	github.com/go-git/go-git/v5 v5.3.0
	github.com/gorilla/csrf v1.7.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/sessions v1.2.1
	github.com/kouhin/envflag v0.0.0-20150818174321-0e9a86061649
	github.com/litao91/goldmark-mathjax v0.0.0-20210217064022-a43cf739a50f
	github.com/yalue/merged_fs v1.0.5
	github.com/yuin/goldmark v1.3.3
	github.com/yuin/goldmark-highlighting v0.0.0-20200307114337-60d527fdb691
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/text v0.3.5 // indirect
)

replace github.com/litao91/goldmark-mathjax v0.0.0-20210217064022-a43cf739a50f => github.com/csmith/goldmark-mathjax v0.0.0-20210331090840-083b73b9825f
