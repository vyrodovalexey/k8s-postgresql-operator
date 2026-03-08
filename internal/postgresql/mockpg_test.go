/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package postgresql

import (
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
)

// mockPGResponse defines how the mock server should respond to a query
type mockPGResponse struct {
	isSelect bool   // true if the query returns rows
	isError  bool   // true if the server should return an error
	errorMsg string // error message to return
	value    string // value to return in the data row
}

// mockPGServer is a minimal PostgreSQL wire protocol server for testing
type mockPGServer struct {
	listener net.Listener
	t        *testing.T
	handler  func(query string) mockPGResponse
	wg       sync.WaitGroup
	closed   bool
	mu       sync.Mutex
}

// newMockPGServer creates and starts a mock PostgreSQL server
func newMockPGServer(t *testing.T, handler func(query string) mockPGResponse) *mockPGServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock PG server: %v", err)
	}

	srv := &mockPGServer{
		listener: ln,
		t:        t,
		handler:  handler,
	}

	srv.wg.Add(1)
	go srv.acceptLoop()

	return srv
}

func (s *mockPGServer) port() int32 {
	return int32(s.listener.Addr().(*net.TCPAddr).Port)
}

func (s *mockPGServer) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.listener.Close()
	s.wg.Wait()
}

func (s *mockPGServer) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

func (s *mockPGServer) handleConn(conn net.Conn) {
	defer conn.Close()

	// Read startup message
	var msgLen int32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err != nil {
		return
	}
	buf := make([]byte, msgLen-4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return
	}

	// Handle SSL request
	if msgLen == 8 {
		proto := binary.BigEndian.Uint32(buf)
		if proto == 80877103 {
			conn.Write([]byte{'N'}) // SSL not supported
			if err := binary.Read(conn, binary.BigEndian, &msgLen); err != nil {
				return
			}
			buf = make([]byte, msgLen-4)
			if _, err := io.ReadFull(conn, buf); err != nil {
				return
			}
		}
	}

	// Send AuthenticationOk
	writeMsg(conn, 'R', []byte{0, 0, 0, 0})

	// Send required parameter status messages
	writeParamStatus(conn, "server_version", "14.0")
	writeParamStatus(conn, "client_encoding", "UTF8")
	writeParamStatus(conn, "standard_conforming_strings", "on")

	// Send BackendKeyData
	writeMsg(conn, 'K', []byte{0, 0, 4, 210, 0, 0, 22, 46})

	// Send ReadyForQuery
	writeMsg(conn, 'Z', []byte{'I'})

	var lastQuery string

	for {
		var mt [1]byte
		if _, err := io.ReadFull(conn, mt[:]); err != nil {
			return
		}

		if err := binary.Read(conn, binary.BigEndian, &msgLen); err != nil {
			return
		}
		buf = make([]byte, msgLen-4)
		if msgLen > 4 {
			if _, err := io.ReadFull(conn, buf); err != nil {
				return
			}
		}

		switch mt[0] {
		case 'Q': // Simple query
			query := string(buf[:len(buf)-1])
			s.handleSimpleQuery(conn, query)
		case 'P': // Parse
			idx := 0
			for idx < len(buf) && buf[idx] != 0 {
				idx++
			}
			idx++ // skip null terminator of statement name
			qEnd := idx
			for qEnd < len(buf) && buf[qEnd] != 0 {
				qEnd++
			}
			lastQuery = string(buf[idx:qEnd])
			// Send ParseComplete
			writeMsg(conn, '1', nil)
		case 'B': // Bind
			// Send BindComplete
			writeMsg(conn, '2', nil)
		case 'D': // Describe
			descType := buf[0]
			if descType == 'S' {
				// Send ParameterDescription
				numParams := strings.Count(lastQuery, "$")
				pd := make([]byte, 2+4*numParams)
				binary.BigEndian.PutUint16(pd, uint16(numParams))
				for i := 0; i < numParams; i++ {
					binary.BigEndian.PutUint32(pd[2+4*i:], 25) // text OID
				}
				writeMsg(conn, 't', pd)
			}

			resp := s.handler(lastQuery)
			if resp.isError {
				sendErrorResponse(conn, resp.errorMsg)
			} else if resp.isSelect {
				sendRowDesc(conn, "result", 25)
			} else {
				writeMsg(conn, 'n', nil) // NoData
			}
		case 'E': // Execute
			resp := s.handler(lastQuery)
			if resp.isError {
				sendErrorResponse(conn, resp.errorMsg)
			} else if resp.isSelect {
				sendDataRow(conn, resp.value)
				writeMsg(conn, 'C', []byte("SELECT 1\x00"))
			} else {
				writeMsg(conn, 'C', []byte("OK\x00"))
			}
		case 'S': // Sync
			writeMsg(conn, 'Z', []byte{'I'})
		case 'X': // Terminate
			return
		}
	}
}

func (s *mockPGServer) handleSimpleQuery(conn net.Conn, query string) {
	resp := s.handler(query)
	if resp.isError {
		sendErrorResponse(conn, resp.errorMsg)
	} else if resp.isSelect {
		sendRowDesc(conn, "result", 25)
		sendDataRow(conn, resp.value)
		writeMsg(conn, 'C', []byte("SELECT 1\x00"))
	} else {
		writeMsg(conn, 'C', []byte("OK\x00"))
	}
	writeMsg(conn, 'Z', []byte{'I'})
}

func writeMsg(conn net.Conn, msgType byte, data []byte) {
	if data == nil {
		data = []byte{}
	}
	buf := make([]byte, 5+len(data))
	buf[0] = msgType
	binary.BigEndian.PutUint32(buf[1:5], uint32(4+len(data)))
	copy(buf[5:], data)
	conn.Write(buf)
}

func writeParamStatus(conn net.Conn, key, value string) {
	data := append([]byte(key), 0)
	data = append(data, []byte(value)...)
	data = append(data, 0)
	writeMsg(conn, 'S', data)
}

func sendRowDesc(conn net.Conn, name string, typeOID uint32) {
	var data []byte
	data = append(data, 0, 1) // 1 field
	data = append(data, []byte(name)...)
	data = append(data, 0)          // null terminator
	data = append(data, 0, 0, 0, 0) // table OID
	data = append(data, 0, 0)       // column number
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, typeOID)
	data = append(data, b...)                   // type OID
	data = append(data, 0xFF, 0xFF)             // type size (-1)
	data = append(data, 0xFF, 0xFF, 0xFF, 0xFF) // type modifier (-1)
	data = append(data, 0, 0)                   // format code (text)
	writeMsg(conn, 'T', data)
}

func sendDataRow(conn net.Conn, value string) {
	var data []byte
	data = append(data, 0, 1) // 1 column
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(len(value)))
	data = append(data, b...)             // data length
	data = append(data, []byte(value)...) // data
	writeMsg(conn, 'D', data)
}

func sendErrorResponse(conn net.Conn, msg string) {
	var data []byte
	data = append(data, 'S')
	data = append(data, []byte("ERROR")...)
	data = append(data, 0)
	data = append(data, 'M')
	data = append(data, []byte(msg)...)
	data = append(data, 0)
	data = append(data, 'C')
	data = append(data, []byte("XX000")...)
	data = append(data, 0)
	data = append(data, 0) // terminator
	writeMsg(conn, 'E', data)
}
