package network

import (
	"net"
)

type Connection struct {
	Addr net.Conn
	Conn *net.Conn
}
