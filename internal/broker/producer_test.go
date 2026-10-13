package broker

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// frame is one PRODUCE request as the broker receives it.
type frame struct {
	topic   string
	key     string
	value   string
	headers map[string]string
}

// readFrame parses the go-kafka wire format:
// [api u8][topic u16-str][key u32-bytes][n u8]{[k u16-str][v u32-bytes]}[value u32-bytes].
func readFrame(r *bufio.Reader) (frame, error) {
	var f frame
	var api uint8
	if err := binary.Read(r, binary.BigEndian, &api); err != nil {
		return f, err
	}
	topic, err := readStr(r)
	if err != nil {
		return f, err
	}
	key, err := readBytes(r)
	if err != nil {
		return f, err
	}
	n, err := r.ReadByte()
	if err != nil {
		return f, err
	}
	f.headers = make(map[string]string, n)
	for range n {
		k, err := readStr(r)
		if err != nil {
			return f, err
		}
		v, err := readBytes(r)
		if err != nil {
			return f, err
		}
		f.headers[k] = string(v)
	}
	value, err := readBytes(r)
	if err != nil {
		return f, err
	}
	f.topic, f.key, f.value = topic, string(key), string(value)
	return f, nil
}

func readStr(r io.Reader) (string, error) {
	var l uint16
	if err := binary.Read(r, binary.BigEndian, &l); err != nil {
		return "", err
	}
	b := make([]byte, l)
	_, err := io.ReadFull(r, b)
	return string(b), err
}

func readBytes(r io.Reader) ([]byte, error) {
	var l uint32
	if err := binary.Read(r, binary.BigEndian, &l); err != nil {
		return nil, err
	}
	b := make([]byte, l)
	_, err := io.ReadFull(r, b)
	return b, err
}

func writeAck(t *testing.T, w io.Writer, code int16) {
	t.Helper()
	bw := bufio.NewWriter(w)
	require.NoError(t, binary.Write(bw, binary.BigEndian, code))
	if code == 0 {
		require.NoError(t, binary.Write(bw, binary.BigEndian, uint16(0)))
		require.NoError(t, binary.Write(bw, binary.BigEndian, uint64(1)))
	}
	require.NoError(t, bw.Flush())
}

// handler serves one broker connection; connNo starts at 1.
type handler func(t *testing.T, connNo int32, c net.Conn, frames chan<- frame)

// startBroker runs a fake broker and returns its address and a connection counter.
func startBroker(t *testing.T, h handler, frames chan<- frame) (host, port string, conns *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	conns = new(atomic.Int32)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			n := conns.Add(1)
			go func() {
				defer c.Close()
				h(t, n, c, frames)
			}()
		}
	}()
	host, port, err = net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return host, port, conns
}

var errAny = errors.New("any error")

func TestProducer_Publish(t *testing.T) {
	ackAll := func(t *testing.T, _ int32, c net.Conn, frames chan<- frame) {
		r := bufio.NewReader(c)
		for {
			f, err := readFrame(r)
			if err != nil {
				return
			}
			frames <- f
			writeAck(t, c, 0)
		}
	}

	tests := []struct {
		name       string
		broker     bool // false: nothing listens on the port
		handle     handler
		timeout    time.Duration
		publishes  int
		wantErrs   []error // per publish: nil, errAny, or a sentinel for errors.Is
		wantConns  int32
		wantFrames int
	}{
		{
			name:       "ack: frame carries topic, key, value and headers",
			broker:     true,
			handle:     ackAll,
			timeout:    time.Second,
			publishes:  1,
			wantErrs:   []error{nil},
			wantConns:  1,
			wantFrames: 1,
		},
		{
			name:       "connection is reused across publishes",
			broker:     true,
			handle:     ackAll,
			timeout:    time.Second,
			publishes:  3,
			wantErrs:   []error{nil, nil, nil},
			wantConns:  1,
			wantFrames: 3,
		},
		{
			name:   "broker error drops the connection, next publish reconnects",
			broker: true,
			handle: func(t *testing.T, connNo int32, c net.Conn, frames chan<- frame) {
				if connNo == 1 {
					_, _ = readFrame(bufio.NewReader(c))
					writeAck(t, c, 2) // broker error code
					return
				}
				ackAll(t, connNo, c, frames)
			},
			timeout:    time.Second,
			publishes:  2,
			wantErrs:   []error{errAny, nil},
			wantConns:  2,
			wantFrames: 1,
		},
		{
			name:   "no answer: publish times out, next publish reconnects",
			broker: true,
			handle: func(t *testing.T, connNo int32, c net.Conn, frames chan<- frame) {
				if connNo == 1 {
					_, _ = readFrame(bufio.NewReader(c))
					_, _ = io.Copy(io.Discard, c) // hang until the client closes
					return
				}
				ackAll(t, connNo, c, frames)
			},
			timeout:    100 * time.Millisecond,
			publishes:  2,
			wantErrs:   []error{context.DeadlineExceeded, nil},
			wantConns:  2,
			wantFrames: 1,
		},
		{
			name:      "nothing listens: connect error",
			broker:    false,
			timeout:   time.Second,
			publishes: 1,
			wantErrs:  []error{errAny},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames := make(chan frame, 16)
			var host, port string
			var conns *atomic.Int32
			if tt.broker {
				host, port, conns = startBroker(t, tt.handle, frames)
			} else {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				host, port, err = net.SplitHostPort(ln.Addr().String())
				require.NoError(t, err)
				require.NoError(t, ln.Close())
			}

			p := NewProducer(host, port, "companies", tt.timeout)
			t.Cleanup(func() { _ = p.Close() })

			headers := map[string]string{"event-type": "company.created", "event-id": "e-1"}
			for i := range tt.publishes {
				err := p.Publish(context.Background(), []byte("agg-1"), []byte(`{"n":1}`), headers)
				switch want := tt.wantErrs[i]; {
				case want == nil:
					require.NoError(t, err, "publish #%d", i+1)
				case errors.Is(want, errAny):
					require.Error(t, err, "publish #%d", i+1)
					assert.Nil(t, p.w, "connection must be dropped after an error")
				default:
					require.ErrorIs(t, err, want, "publish #%d", i+1)
					assert.Nil(t, p.w, "connection must be dropped after an error")
				}
			}

			if conns != nil {
				assert.Equal(t, tt.wantConns, conns.Load(), "connections opened")
			}
			close(frames)
			got := make([]frame, 0, tt.wantFrames)
			for f := range frames {
				got = append(got, f)
			}
			require.Len(t, got, tt.wantFrames)
			for _, f := range got {
				assert.Equal(t, frame{topic: "companies", key: "agg-1", value: `{"n":1}`, headers: headers}, f)
			}
		})
	}
}
