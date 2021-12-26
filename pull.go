package hdl

import (
	"errors"
	"io"
	"net/http"
	"time"

	. "github.com/Monibuca/engine/v3"
	"github.com/Monibuca/utils/v3/codec"
)

func PullStream(streamPath, url string) error {
	if res, err := http.Get(url); err == nil {
		stream := Stream{
			Type:       "HDL Pull",
			StreamPath: streamPath,
		}
		if stream.Publish() {
			defer stream.Close()
			head := make([]byte, len(codec.FLVHeader))
			io.ReadFull(res.Body, head)
			var lastTime uint32
			at := stream.NewAudioTrack(0)
			vt := stream.NewVideoTrack(0)
			for {
				if t, timestamp, payload, err := codec.ReadFLVTag(res.Body); err == nil {
					switch t {
					case codec.FLV_TAG_TYPE_AUDIO:
						at.PushByteStream(timestamp, payload)
					case codec.FLV_TAG_TYPE_VIDEO:
						if timestamp != 0 {
							if lastTime == 0 {
								lastTime = timestamp
							}
						}
						vt.PushByteStream(timestamp, payload)
						time.Sleep(time.Duration(timestamp-lastTime) * time.Millisecond)
						lastTime = timestamp
					}
				} else {
					return err
				}
			}
		} else {
			return errors.New("Bad Name")
		}
	} else {
		return err
	}
}
