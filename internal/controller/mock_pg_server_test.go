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

package controller

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
)

// mockPGServer is a minimal PostgreSQL wire protocol server for testing.
// It accepts connections and responds to queries with configurable results.
type mockPGServer struct {
	listener net.Listener
	port     int32
	// queryResponses maps query substrings to response behaviors
	// If a query contains the key, the corresponding handler is called
	queryResponses map[string]func(conn net.Conn, query string) error
	// defaultResponse is used when no queryResponses match
	defaultSuccess bool
	done           chan struct{}
	t              *testing.T
}

// newMockPGServer creates and starts a mock PostgreSQL server.
// It returns the server and the port it's listening on.
func newMockPGServer(t *testing.T, defaultSuccess bool) *mockPGServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	addr := listener.Addr().(*net.TCPAddr)
	srv := &mockPGServer{
		listener:       listener,
		port:           int32(addr.Port),
		queryResponses: make(map[string]func(conn net.Conn, query string) error),
		defaultSuccess: defaultSuccess,
		done:           make(chan struct{}),
		t:              t,
	}

	go srv.serve()
	return srv
}

func (s *mockPGServer) close() {
	s.listener.Close()
	<-s.done
}

func (s *mockPGServer) serve() {
	defer close(s.done)
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
		go s.handleConnection(conn)
	}
}

func (s *mockPGServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read startup message
	if err := s.readStartupMessage(conn); err != nil {
		return
	}

	// Send AuthenticationOk
	s.sendAuthOk(conn)

	// Send ParameterStatus messages
	s.sendParameterStatus(conn, "server_version", "15.0")
	s.sendParameterStatus(conn, "client_encoding", "UTF8")
	s.sendParameterStatus(conn, "server_encoding", "UTF8")
	s.sendParameterStatus(conn, "DateStyle", "ISO, MDY")
	s.sendParameterStatus(conn, "TimeZone", "UTC")
	s.sendParameterStatus(conn, "standard_conforming_strings", "on")

	// Send BackendKeyData
	s.sendBackendKeyData(conn)

	// Send ReadyForQuery
	s.sendReadyForQuery(conn)

	// Handle queries - track last parsed query for extended protocol
	var lastQuery string

	for {
		msgType, payload, err := s.readMessage(conn)
		if err != nil {
			return
		}

		switch msgType {
		case 'Q': // Simple query
			query := string(payload[:len(payload)-1]) // Remove null terminator
			s.handleQuery(conn, query)
		case 'X': // Terminate
			return
		case 'P': // Parse (extended query protocol)
			// Extract query from Parse payload: statement_name\0 query\0 int16(numParams) int32[](paramOIDs)
			idx := 0
			for idx < len(payload) && payload[idx] != 0 {
				idx++
			}
			idx++ // skip null terminator of statement name
			qEnd := idx
			for qEnd < len(payload) && payload[qEnd] != 0 {
				qEnd++
			}
			lastQuery = string(payload[idx:qEnd])
			// Send ParseComplete
			conn.Write([]byte{'1', 0, 0, 0, 4})
		case 'B': // Bind
			// Send BindComplete
			conn.Write([]byte{'2', 0, 0, 0, 4})
		case 'D': // Describe
			descType := byte('S')
			if len(payload) > 0 {
				descType = payload[0]
			}
			if descType == 'S' {
				// Send ParameterDescription with the correct number of parameters
				numParams := strings.Count(lastQuery, "$")
				pd := make([]byte, 2+4*numParams)
				binary.BigEndian.PutUint16(pd, uint16(numParams))
				for i := 0; i < numParams; i++ {
					binary.BigEndian.PutUint32(pd[2+4*i:], 25) // text OID
				}
				s.writeMsg(conn, 't', pd)
			}

			// Determine response type based on query
			queryUpper := strings.ToUpper(strings.TrimSpace(lastQuery))
			if strings.Contains(queryUpper, "SELECT") {
				s.sendRowDescription(conn, []string{"exists"})
			} else {
				// NoData
				s.writeMsg(conn, 'n', nil)
			}
		case 'E': // Execute
			queryUpper := strings.ToUpper(strings.TrimSpace(lastQuery))
			if strings.Contains(queryUpper, "SELECT") {
				if s.defaultSuccess {
					s.sendDataRow(conn, []string{"t"})
				} else {
					s.sendDataRow(conn, []string{"f"})
				}
				s.sendCommandComplete(conn, "SELECT 1")
			} else {
				s.sendCommandComplete(conn, "OK")
			}
		case 'S': // Sync
			s.sendReadyForQuery(conn)
		default:
			// Unknown message, send error
			s.sendErrorResponse(conn, "ERROR", "XX000",
				fmt.Sprintf("unsupported message type: %c", msgType))
			s.sendReadyForQuery(conn)
		}
	}
}

func (s *mockPGServer) readStartupMessage(conn net.Conn) error {
	// Read length (4 bytes)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return err
	}
	length := binary.BigEndian.Uint32(lenBuf)

	// Read the rest of the startup message
	if length > 4 {
		buf := make([]byte, length-4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return err
		}

		// Check for SSL request (80877103)
		if length == 8 {
			version := binary.BigEndian.Uint32(buf[:4])
			if version == 80877103 {
				// SSL request - respond with 'N' (no SSL)
				conn.Write([]byte{'N'})
				// Read the actual startup message
				return s.readStartupMessage(conn)
			}
		}
	}
	return nil
}

func (s *mockPGServer) readMessage(conn net.Conn) (byte, []byte, error) {
	// Read message type (1 byte)
	typeBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, typeBuf); err != nil {
		return 0, nil, err
	}

	// Read length (4 bytes)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)

	// Read payload
	var payload []byte
	if length > 4 {
		payload = make([]byte, length-4)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, err
		}
	}

	return typeBuf[0], payload, nil
}

func (s *mockPGServer) sendAuthOk(conn net.Conn) {
	// AuthenticationOk: 'R' + length(8) + auth_type(0)
	msg := []byte{'R', 0, 0, 0, 8, 0, 0, 0, 0}
	conn.Write(msg)
}

func (s *mockPGServer) sendParameterStatus(conn net.Conn, name, value string) {
	// ParameterStatus: 'S' + length + name\0 + value\0
	payload := append([]byte(name), 0)
	payload = append(payload, []byte(value)...)
	payload = append(payload, 0)

	length := uint32(4 + len(payload))
	msg := []byte{'S'}
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, length)
	msg = append(msg, lenBytes...)
	msg = append(msg, payload...)
	conn.Write(msg)
}

func (s *mockPGServer) sendBackendKeyData(conn net.Conn) {
	// BackendKeyData: 'K' + length(12) + pid(4) + secret(4)
	msg := []byte{'K', 0, 0, 0, 12, 0, 0, 0, 1, 0, 0, 0, 1}
	conn.Write(msg)
}

func (s *mockPGServer) sendReadyForQuery(conn net.Conn) {
	// ReadyForQuery: 'Z' + length(5) + status('I' = idle)
	msg := []byte{'Z', 0, 0, 0, 5, 'I'}
	conn.Write(msg)
}

func (s *mockPGServer) sendErrorResponse(conn net.Conn, severity, code, message string) {
	// ErrorResponse: 'E' + length + fields
	var payload []byte
	payload = append(payload, 'S')
	payload = append(payload, []byte(severity)...)
	payload = append(payload, 0)
	payload = append(payload, 'V')
	payload = append(payload, []byte(severity)...)
	payload = append(payload, 0)
	payload = append(payload, 'C')
	payload = append(payload, []byte(code)...)
	payload = append(payload, 0)
	payload = append(payload, 'M')
	payload = append(payload, []byte(message)...)
	payload = append(payload, 0)
	payload = append(payload, 0) // terminator

	length := uint32(4 + len(payload))
	msg := []byte{'E'}
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, length)
	msg = append(msg, lenBytes...)
	msg = append(msg, payload...)
	conn.Write(msg)
}

func (s *mockPGServer) sendCommandComplete(conn net.Conn, tag string) {
	// CommandComplete: 'C' + length + tag\0
	payload := append([]byte(tag), 0)
	length := uint32(4 + len(payload))
	msg := []byte{'C'}
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, length)
	msg = append(msg, lenBytes...)
	msg = append(msg, payload...)
	conn.Write(msg)
}

func (s *mockPGServer) sendRowDescription(conn net.Conn, columns []string) {
	// RowDescription: 'T' + length + numFields(2) + fields...
	var payload []byte
	numFields := make([]byte, 2)
	binary.BigEndian.PutUint16(numFields, uint16(len(columns)))
	payload = append(payload, numFields...)

	for _, col := range columns {
		payload = append(payload, []byte(col)...)
		payload = append(payload, 0)
		// table OID (4), column attr number (2), data type OID (4), data type size (2),
		// type modifier (4), format code (2) = 18 bytes
		payload = append(payload, make([]byte, 18)...)
		// Set data type OID to 16 (bool) for simplicity
		binary.BigEndian.PutUint32(payload[len(payload)-14:len(payload)-10], 16)
		binary.BigEndian.PutUint16(payload[len(payload)-10:len(payload)-8], 1)
	}

	length := uint32(4 + len(payload))
	msg := []byte{'T'}
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, length)
	msg = append(msg, lenBytes...)
	msg = append(msg, payload...)
	conn.Write(msg)
}

func (s *mockPGServer) sendDataRow(conn net.Conn, values []string) {
	// DataRow: 'D' + length + numColumns(2) + (colLength(4) + colData)...
	var payload []byte
	numCols := make([]byte, 2)
	binary.BigEndian.PutUint16(numCols, uint16(len(values)))
	payload = append(payload, numCols...)

	for _, val := range values {
		colLen := make([]byte, 4)
		binary.BigEndian.PutUint32(colLen, uint32(len(val)))
		payload = append(payload, colLen...)
		payload = append(payload, []byte(val)...)
	}

	length := uint32(4 + len(payload))
	msg := []byte{'D'}
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, length)
	msg = append(msg, lenBytes...)
	msg = append(msg, payload...)
	conn.Write(msg)
}

func (s *mockPGServer) handleQuery(conn net.Conn, query string) {
	queryUpper := strings.ToUpper(strings.TrimSpace(query))

	// Check custom handlers
	for pattern, handler := range s.queryResponses {
		if strings.Contains(queryUpper, strings.ToUpper(pattern)) {
			if err := handler(conn, query); err != nil {
				s.sendErrorResponse(conn, "ERROR", "XX000", err.Error())
			}
			s.sendReadyForQuery(conn)
			return
		}
	}

	// Default handling
	if strings.Contains(queryUpper, "SELECT EXISTS") || strings.Contains(queryUpper, "SELECT 1") {
		// Return a single boolean row
		s.sendRowDescription(conn, []string{"exists"})
		if s.defaultSuccess {
			s.sendDataRow(conn, []string{"t"})
		} else {
			s.sendDataRow(conn, []string{"f"})
		}
		s.sendCommandComplete(conn, "SELECT 1")
	} else if strings.Contains(queryUpper, "SELECT") {
		// Empty result set
		s.sendRowDescription(conn, []string{"result"})
		s.sendCommandComplete(conn, "SELECT 0")
	} else if strings.Contains(queryUpper, "CREATE") ||
		strings.Contains(queryUpper, "ALTER") ||
		strings.Contains(queryUpper, "GRANT") ||
		strings.Contains(queryUpper, "DROP") ||
		strings.Contains(queryUpper, "SET") ||
		strings.Contains(queryUpper, "DELETE") {
		s.sendCommandComplete(conn, "OK")
	} else {
		s.sendCommandComplete(conn, "OK")
	}
	s.sendReadyForQuery(conn)
}

// writeMsg writes a PostgreSQL wire protocol message.
func (s *mockPGServer) writeMsg(conn net.Conn, msgType byte, data []byte) {
	if data == nil {
		data = []byte{}
	}
	buf := make([]byte, 5+len(data))
	buf[0] = msgType
	binary.BigEndian.PutUint32(buf[1:5], uint32(4+len(data)))
	copy(buf[5:], data)
	conn.Write(buf)
}
