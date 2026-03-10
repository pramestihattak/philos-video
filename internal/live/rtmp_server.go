package live

import (
	"fmt"
	"io"
	"log/slog"
	"net"

	rtmp "github.com/yutopp/go-rtmp"
)

// RTMPServer wraps the go-rtmp server and ties it to the Manager.
type RTMPServer struct {
	manager *Manager
}

func NewRTMPServer(manager *Manager) *RTMPServer {
	return &RTMPServer{manager: manager}
}

// ListenAndServe starts the RTMP server on the given port. Blocks until error.
func (s *RTMPServer) ListenAndServe(port int) error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("RTMP listen: %w", err)
	}
	slog.Info("RTMP server listening", "addr", fmt.Sprintf("rtmp://localhost:%d/live", port))

	srv := rtmp.NewServer(&rtmp.ServerConfig{
		OnConnect: func(conn net.Conn) (io.ReadWriteCloser, *rtmp.ConnConfig) {
			return conn, &rtmp.ConnConfig{
				Handler: &rtmpHandler{manager: s.manager},
			}
		},
	})
	return srv.Serve(l)
}
