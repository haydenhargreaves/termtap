package model

import (
	"net"
	"net/http"
)

type ProxyServer struct {
	Listener *net.Listener
	Server   *http.Server
	Url      string
}
