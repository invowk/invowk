// SPDX-License-Identifier: MPL-2.0

package sshserver

import "github.com/invowk/invowk/internal/runtime"

type runtimeHostCallbackServer struct {
	server *Server
}

// NewRuntimeHostCallbackServer adapts an SSH server to the runtime host-callback port.
func NewRuntimeHostCallbackServer(server *Server) runtime.HostCallbackServer {
	if server == nil {
		return nil
	}
	return runtimeHostCallbackServer{server: server}
}

func (s runtimeHostCallbackServer) IsRunning() bool {
	return s.server != nil && s.server.IsRunning()
}

func (s runtimeHostCallbackServer) GetConnectionInfo(commandID runtime.HostCallbackCommandID) (*runtime.HostCallbackConnectionInfo, error) {
	sshCommandID := CommandID(commandID.String())
	if err := sshCommandID.Validate(); err != nil {
		return nil, err
	}
	info, err := s.server.GetConnectionInfo(sshCommandID)
	if err != nil {
		return nil, err
	}
	host := runtime.HostCallbackHost(info.Host.String())
	if err := host.Validate(); err != nil {
		return nil, err
	}
	token := runtime.HostCallbackToken(info.Token.String())
	if err := token.Validate(); err != nil {
		return nil, err
	}
	port := info.Port
	if err := port.Validate(); err != nil {
		return nil, err
	}
	user := runtime.HostCallbackUser(info.User)
	if err := user.Validate(); err != nil {
		return nil, err
	}
	connInfo := &runtime.HostCallbackConnectionInfo{
		Host:  host,
		Port:  port,
		Token: token,
		User:  user,
	}
	if err := connInfo.Validate(); err != nil {
		return nil, err
	}
	return connInfo, nil
}

func (s runtimeHostCallbackServer) RevokeToken(token runtime.HostCallbackToken) {
	sshToken := TokenValue(token.String())
	if err := sshToken.Validate(); err != nil {
		return
	}
	s.server.RevokeToken(sshToken)
}
