package model

import (
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

type ProxyServer struct {
	Listener *net.Listener
	Server   *http.Server
	Url      string
}

type Request struct {
	ID              uuid.UUID
	Method          string
	ResponseData    []byte
	RequestData     []byte
	RawURL          string
	Host            string
	URL             string
	QueryString     string
	QueryMap        url.Values
	Status          int
	Duration        time.Duration
	Pending         bool
	Failed          bool
	StartTime       time.Time
	RequestHeaders  http.Header
	ResponseHeaders http.Header
}
