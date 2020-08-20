package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type TestsFile []Test

// WebSocketTest .
type Test struct {
	// test name, will be used in output filenames etc.
	Name string `json:"name" yaml:"name"`

	// The websocket endpoint to test
	URL string `json:"url" yaml:"url"`

	// if we expect the server to close the connection, code 1000 is for normal
	// close, 1011 for Server error. If left at default value server close will
	// not be checked.
	ExpectServerClose int `json:"expect_server_close" yaml:"expect_server_close"`

	// defaults to 1 second if not specified
	MessageReadTimeout Duration `json:"message_read_timeout" yaml:"message_read_timeout"`

	// defaults to 1 second unless specified
	MessageWriteTimeout Duration `json:"message_write_timeout" yaml:"message_write_timeout"`

	// number of messages to expect the server to send to count the test as successful
	ExpectMessages int `json:"expect_messages" yaml:"expect_messages"`

	// defaults to 30s
	HandshakeTimeout Duration `json:"handshake_timeout" yaml:"handshake_timeout"`

	// If set this message will be sent after connect.
	SendTextMessage string `json:"send_text_message" yaml:"send_text_message"`

	Sleep Duration `json:"sleep" yaml:"sleep"`
}

// WebSocketMessage .
type WebsocketMessage struct {
	ReceivedAt Duration    `json:"received_at,omitempty"`
	Type       int         `json:"type,omitempty"`
	Body       interface{} `json:"body,omitempty"`
}

// Log .
type Log struct {
	CreatedAt Duration    `json:"created_at,omitempty"`
	Kind      string      `json:"kind,omitempty"`
	Msg       string      `json:"message,omitempty"`
	Value     interface{} `json:"value,omitempty"`
	Err       error       `json:"error,omitempty"`
}

const (
	LogConnect                      = "connect"
	LogConnectSuccess               = "connect_success"
	LogConnectFail                  = "connect_fail"
	LogServerClosedConnection       = "server_closed_connection"
	LogSetReadDeadlineFailed        = "set_read_deadline_failed"
	LogSetWriteDeadlineFailed       = "set_write_deadline_failed"
	LogReadMessage                  = "read_message"
	LogReadMessageTimeout           = "read_message_timeout"
	LogReadMessageNetError          = "read_message_net_error"
	LogReadMessageError             = "read_message_error"
	LogReadMessageSuccess           = "read_message_success"
	LogWriteMessage                 = "write_message"
	LogWriteMessageTimeout          = "write_message_timeout"
	LogWriteMessageNetError         = "write_message_net_error"
	LogWriteMessageError            = "write_message_error"
	LogWriteMessageSuccess          = "write_message_success"
	LogClientCloseConnection        = "client_close_connection"
	LogClientCloseConnectionSuccess = "client_close_connection_success"
	LogClientCloseConnectionFailed  = "client_close_connection_failed"
)

type TestResult struct {
	ID               string             `json:"id"`
	Test             Test               `json:"test"` // The associated with the result
	StartedAt        time.Time          `json:"started_at"`
	ConnectOK        bool               `json:"connect_ok"`        // true if connect succeeds
	ConnectDuration  Duration           `json:"connect_duration"`  // set regardless if connect fails or not
	MessagesReceived int                `json:"messages_received"` // number of messages received
	Messages         []WebsocketMessage `json:"messages"`
	ServerCloseCode  int                `json:"server_close_code"`
	CloseOK          bool               `json:"close_ok"`
	Log              []Log              `json:"log"`
}

func (r TestResult) IsSuccess() bool {
	t := r.Test

	if t.ExpectServerClose != 0 && (r.ServerCloseCode != t.ExpectServerClose) {
		return false
	}

	if t.ExpectMessages != 0 && (r.MessagesReceived != t.ExpectMessages) {
		return false
	}

	return true

}

func testWS(ctx context.Context, wt Test) (TestResult, error) {
	// Initialize the check - this will return an UNKNOWN result
	// until more results are added.
	if wt.Name == "" {
		return TestResult{}, errors.New("test name cannot be empty")
	}
	uuid, err := uuid.NewRandom()
	if err != nil {
		return TestResult{}, err
	}
	id := uuid.String()

	if wt.MessageReadTimeout == 0 {
		wt.MessageReadTimeout = Duration(time.Second)
	}
	if wt.MessageWriteTimeout == 0 {
		wt.MessageWriteTimeout = Duration(time.Second)
	}
	if wt.HandshakeTimeout == 0 {
		wt.HandshakeTimeout = Duration(30 * time.Second)
	}
	{
		data, err := json.MarshalIndent(&wt, "", "  ")
		if err != nil {
			return TestResult{}, err
		}
		log.Println(id, "new test", string(data))
	}
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: wt.HandshakeTimeout.D(),
	}
	start := time.Now()
	timestamp := func() Duration {
		return Duration(time.Now().Sub(start))
	}

	wr := TestResult{ID: id, Test: wt, StartedAt: start}
	addLog := func(kind string, log ...Log) {
		if len(log) > 1 {
			panic("only one log item supported")
		}
		var l Log
		if len(log) < 1 {
			l = Log{}
		} else {
			l = log[0]
		}

		l.Kind = kind
		if l.CreatedAt == 0 {
			l.CreatedAt = timestamp()
		}

		wr.Log = append(wr.Log, l)
	}

	// Connect to the server
	addLog(LogConnect)
	log.Printf("%s Connecting to %s", wr.ID, wt.URL)
	c, _, err := dialer.Dial(wt.URL, nil)
	wr.ConnectDuration = Duration(time.Now().Sub(start))
	log.Println(wr.ID, "connect duration", wr.ConnectDuration)
	if err != nil {
		addLog(LogConnectFail, Log{Err: err})
		log.Println(wr.ID, "Cannot connect to websocket")
		return wr, nil
	}
	addLog(LogConnectSuccess)
	wr.ConnectOK = true
	c.SetCloseHandler(func(code int, text string) error {
		log.Println(wr.ID, code, text)
		return nil
	})
	log.Println(wr.ID, "connected")
	defer c.Close()

	if wt.SendTextMessage != "" {
		var err error
		addLog(LogWriteMessage)
		err = c.SetWriteDeadline(time.Now().Add(wt.MessageWriteTimeout.D()))
		if err != nil {
			addLog(LogSetWriteDeadlineFailed, Log{Err: err})
			return wr, err
		}

		err = c.WriteMessage(websocket.TextMessage, []byte(wt.SendTextMessage))
		if err != nil {
			spew.Dump(err)
			addLog(LogWriteMessageError, Log{Err: err})
			if err, ok := err.(*websocket.CloseError); ok {
				log.Println(wr.ID, err.Code)
			}
			log.Println(wr.ID, "Error while closing websocket", err)
			return wr, nil

		}
		addLog(LogWriteMessageSuccess)
	}

	if wt.ExpectMessages < 1 {
		if err := c.SetReadDeadline(time.Now().Add(wt.MessageReadTimeout.D())); err != nil {
			addLog(LogSetReadDeadlineFailed, Log{Err: err})
			return wr, err
		}
		addLog(LogReadMessage)
		msgType, data, err := c.ReadMessage()
		if err != nil {
			switch err := err.(type) {
			case *websocket.CloseError:
				addLog(LogServerClosedConnection, Log{Err: err})
				log.Println(wr.ID, "connection closed by server", msgType, err.Code, err.Text)
				wr.ServerCloseCode = err.Code
				return wr, nil
			case net.Error:
				if err.Timeout() {
					addLog(LogReadMessageTimeout)
				} else {
					addLog(LogReadMessageNetError, Log{Err: err})
				}
			default:
				addLog(LogReadMessageError, Log{Err: err})
				spew.Dump(err)
				log.Println(wr.ID, err)
			}
		} else {
			addLog(LogReadMessageSuccess, Log{Value: msgType})
			wr.MessagesReceived = wr.MessagesReceived + 1
			if msgType == websocket.BinaryMessage {
				wr.Messages = append(wr.Messages, WebsocketMessage{
					Type:       msgType,
					Body:       data,
					ReceivedAt: timestamp(),
				})
			} else {
				wr.Messages = append(wr.Messages, WebsocketMessage{
					Type:       msgType,
					Body:       string(data),
					ReceivedAt: timestamp(),
				})
			}
			log.Println(wr.ID, string(data))
		}
	} else {
		for wr.MessagesReceived < wt.ExpectMessages {
			if err := c.SetReadDeadline(time.Now().Add(wt.MessageReadTimeout.D())); err != nil {
				addLog(LogSetReadDeadlineFailed, Log{Err: err})
				return wr, err
			}
			msgType, data, err := c.ReadMessage()
			if err != nil {

				switch err := err.(type) {
				case *websocket.CloseError:
					addLog(LogServerClosedConnection, Log{Err: err})
					log.Println(wr.ID, "connection closed by server", msgType, err.Code, err.Text)
					wr.ServerCloseCode = err.Code
					return wr, nil
				case net.Error:
					if err.Timeout() {
						addLog(LogReadMessageTimeout)
					} else {
						addLog(LogReadMessageNetError, Log{Err: err})
					}
				default:
					addLog(LogReadMessageError, Log{Err: err})
					log.Println(wr.ID, err)
				}
			} else {
				addLog(LogReadMessageSuccess, Log{Value: msgType})
				log.Println(wr.ID, string(data))
				wr.Messages = append(wr.Messages, WebsocketMessage{
					Type:       msgType,
					Body:       data,
					ReceivedAt: timestamp(),
				})
				wr.MessagesReceived = wr.MessagesReceived + 1
			}
		}
	}

	// close the connection
	addLog(LogClientCloseConnection)
	log.Println(wr.ID, "Requesting connection closure")
	err = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		addLog(LogClientCloseConnectionFailed, Log{Err: err})
		if err, ok := err.(*websocket.CloseError); ok {
			log.Println(wr.ID, err.Code)
		}
		log.Println(wr.ID, "Error while closing websocket", err)
		return wr, nil

	}
	addLog(LogClientCloseConnectionSuccess)
	wr.CloseOK = true
	return wr, nil

}
