//go:build windows
// +build windows

package common

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"

	"github.com/Microsoft/go-winio"
)

func GetSocketName(clusterID string) string {
	return fmt.Sprintf("namedpipe:/\\.\\pipe\\autok3s-%s", md5hash(clusterID))
}

func GetSocketDialer() func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, addr string) (net.Conn, error) {
		clusterID, _, _ := net.SplitHostPort(addr)
		u, _ := url.Parse(GetSocketName(clusterID))
		return winio.DialPipeContext(ctx, u.Path)
	}
}

func md5hash(s string) string {
	hash := md5.Sum([]byte(s))
	hexStr := hex.EncodeToString(hash[:])
	return hexStr[:16]
}
