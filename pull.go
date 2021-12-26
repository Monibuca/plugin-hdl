package hdl

import (
	"errors"
	"io"
	"net/http"
	"time"

	. "github.com/Monibuca/engine/v3"
	"github.com/Monibuca/utils/v3/codec"
)

func pull(stream *Stream, reader io.Reader, lastDisconnect uint32) error {
	var lastTime uint32
	if config.Reconnect {
		time.Sleep(time.Second * 5)
		go pull(stream, reader, lastTime)
	} else {
		defer stream.Close()
	}
	head := make([]byte, len(codec.FLVHeader))
	io.ReadFull(reader, head)
	at := stream.NewAudioTrack(0)
	vt := stream.NewVideoTrack(0)
	for readT := time.Now(); ; readT = time.Now() {
		if t, timestamp, payload, err := codec.ReadFLVTag(reader); err == nil {
			if lastDisconnect != 0 && timestamp == 0 {
				continue
			}
			readCost := time.Since(readT)
			switch t {
			case codec.FLV_TAG_TYPE_AUDIO:
				at.PushByteStream(timestamp+lastDisconnect, payload)
			case codec.FLV_TAG_TYPE_VIDEO:
				vt.PushByteStream(timestamp+lastDisconnect, payload)
				if timestamp != 0 {
					if duration := time.Duration(timestamp-lastTime) * time.Millisecond; readCost < duration {
						time.Sleep(duration - readCost)
					}
				}
				lastTime = timestamp
			}
		} else {
			return err
		}
	}
}
func PullStream(streamPath, url string) error {
	if res, err := http.Get(url); err == nil {
		stream := Stream{
			Type:       "HDL Pull",
			StreamPath: streamPath,
		}
		if stream.Publish() {
			go pull(&stream, res.Body, 0)
		} else {
			return errors.New("Bad Name")
		}
	} else {
		return err
	}
	return nil
}
