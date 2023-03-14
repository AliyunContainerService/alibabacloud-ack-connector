package base

import (
	"context"
	"encoding/binary"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
)

// TunnelEndpoint is initially designed for common parts of stub and agent.
type TunnelEndpoint struct {
	Component
}

const (
	SessionIDHeaderKey = "X-Tunnel-Session-ID"
	BufferSize         = 4096
)

func NewTunnelEndpoint(ctx context.Context, logger *logrus.Logger) TunnelEndpoint {
	return TunnelEndpoint{
		Component: NewComponent(ctx, logger),
	}
}

// CheckAndStartPipe will check request header and response status code to see if there is a successful upgrade negotiation.
// The order of endpointA and endpointB only affect to log, A2B is forward and B2A is backward. X2Y means read from X and write to Y.
// This pipe is a full duplex pipe. The close of this pipe is caused by either context done or any error from any connection.
// The data is read in SPDY format where the auxiliary function `readSPDYFrame` does.
// Reference: https://www.chromium.org/spdy/spdy-protocol/spdy-protocol-draft3-1 section 2.2.
func (endpoint *TunnelEndpoint) CheckAndStartPipe(request *http.Request, response *http.Response, endpointA io.ReadWriter, endpointB io.ReadWriter) {
	if request.Header.Get("Upgrade") == "SPDY/3.1" && response.StatusCode == 101 {
		logger := endpoint.Logger.WithField(SessionIDHeaderKey, request.Header.Get(SessionIDHeaderKey))
		logger.Tracef("Upgrade to protocol %s, SPDY pipe start", request.Header.Get("Upgrade"))
		ctx, cancel := context.WithCancel(endpoint.Context)
		pipe := func(r io.Reader, w io.Writer, log *logrus.Entry) {
			log.Tracef("Start pipe")
			defer cancel()
			for {
				select {
				case <-ctx.Done():
					log.Tracef("Exit")
					return
				default:
					log.Tracef("Reading")
					bytes, err := endpoint.readSPDYFrame(r, log)
					if err != nil {
						log.Tracef("Read failed: ", err)
						return
					}
					log.Tracef("Read completed")
					n, err := w.Write(bytes)
					if err != nil {
						log.Tracef("Write failed: ", err)
						return
					}
					log.Tracef("Write completed, %d bytes transferred.", n)
				}
			}
		}
		go pipe(endpointA, endpointB, logger.WithField("pipe", "forward"))
		go pipe(endpointB, endpointA, logger.WithField("pipe", "backward"))
		<-ctx.Done()
	}

	if request.Header.Get("Upgrade") == "websocket" && response.StatusCode == 101 {
		logger := endpoint.Logger.WithField(SessionIDHeaderKey, request.Header.Get(SessionIDHeaderKey))
		logger.Tracef("Upgrade to protocol %s, websocket pipe start", request.Header.Get("Upgrade"))
		ctx, cancel := context.WithCancel(endpoint.Context)
		pipe := func(r io.Reader, w io.Writer, log *logrus.Entry) {
			defer cancel()
			for {
				select {
				case <-ctx.Done():
					log.Debugf("Exit")
					return
				default:
					log.Tracef("Reading")
					bytes, err := endpoint.readWebsocketFrame(r, log)
					if err != nil {
						log.Tracef("Read failed: ", err)
						return
					}
					log.Tracef("Read completed")
					n, err := w.Write(bytes)
					if err != nil {
						log.Tracef("Write failed: ", err)
						return
					}
					log.Tracef("Write completed, %d bytes transferred.", n)
				}
			}
		}
		go pipe(endpointA, endpointB, logger.WithField("pipe", "forward"))
		go pipe(endpointB, endpointA, logger.WithField("pipe", "backward"))
		<-ctx.Done()
	}
}

func (endpoint *TunnelEndpoint) readSPDYFrame(reader io.Reader, log *logrus.Entry) ([]byte, error) {
	log.Debugf("[ReadFrame] start read SPDY Frame")
	header := make([]byte, 8)
	cnt := 0
	log.Debugf("[ReadFrame] reading header")
	for cnt < 8 {
		n, err := reader.Read(header[cnt:])
		if err != nil {
			return nil, err
		}
		log.Debugf("[ReadFrame] reading head %d bytes", n)
		cnt += n
	}
	log.Debugf("[ReadFrame] header read complete")
	length := int(binary.BigEndian.Uint32(append([]byte{0}, header[5:8]...))) + 8
	frame := make([]byte, length)
	copy(frame, header)
	log.Debugf("[ReadFrame] reading body [%d bytes]", length-8)
	for cnt < length {
		n, err := reader.Read(frame[cnt:])
		if err != nil {
			return nil, err
		}
		cnt += n
	}
	log.Debugf("[ReadFrame] body read complete")
	return frame, nil
}

func readFrame(size int, reader io.Reader, log *logrus.Entry) ([]byte, error) {
	data := make([]byte, 0)
	for {
		if len(data) == size {
			break
		}
		// Temporary slice to read chunk
		sz := BufferSize
		remaining := size - len(data)
		if sz > remaining {
			sz = remaining
		}
		temp := make([]byte, sz)

		n, err := reader.Read(temp)
		if err != nil && err != io.EOF {
			return data, err
		}
		data = append(data, temp[:n]...)
	}
	return data, nil
}

func (endpoint *TunnelEndpoint) readWebsocketFrame(reader io.Reader, log *logrus.Entry) ([]byte, error) {
	log.Tracef("[ReadFrame] start")
	head, err := readFrame(2, reader, log)
	if err != nil {
		return nil, err
	}

	result := make([]byte, 0)
	result = append(result, head[:2]...)
	IsMasked := (head[1] & 0x80) == 0x80

	var length uint64
	length = uint64(head[1] & 0x7F)

	if length == 126 {
		lenBytes, err := readFrame(2, reader, log)
		if err != nil {
			return nil, err
		}
		length = uint64(binary.BigEndian.Uint16(lenBytes))
		result = append(result, lenBytes[:2]...)
	} else if length == 127 {
		lenBytes, err := readFrame(8, reader, log)
		if err != nil {
			return nil, err
		}
		length = binary.BigEndian.Uint64(lenBytes)
		result = append(result, lenBytes[:8]...)
	}
	if IsMasked {
		maskKey, err := readFrame(4, reader, log)
		if err != nil {
			return nil, err
		}
		result = append(result, maskKey[:4]...)
	}

	payload, err := readFrame(int(length), reader, log) // possible data loss
	if err != nil {
		log.Errorf("[ReadFrame] payload err: %v", err)
		return nil, err
	}
	result = append(result, payload[:length]...)
	log.Tracef("[ReadFrame] payload append")
	return result, err
}
