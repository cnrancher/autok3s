//go:build unix
// +build unix

package common

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
)

const (
	socketFileName = "socket.sock"
)

func GetSocketName(clusterID string) string {
	return fmt.Sprintf("unix://%s", filepath.Join(GetClusterContextPath(clusterID), socketFileName))
}

func GetSocketDialer() func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, addr string) (net.Conn, error) {
		clusterID, _, _ := net.SplitHostPort(addr)
		u, _ := url.Parse(GetSocketName(clusterID))
		var d net.Dialer
		return d.DialContext(ctx, "unix", u.Path)
	}
}
