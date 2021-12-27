package hdl

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	. "github.com/Monibuca/engine/v3"
	"github.com/Monibuca/utils/v3/codec"
)

func pull(at *AudioTrack, vt *VideoTrack, reader io.Reader, lastDisconnect uint32) (lastTime uint32) {
	head := make([]byte, len(codec.FLVHeader))
	io.ReadFull(reader, head)
	for startTime := time.Now(); ; {
		if t, timestamp, payload, err := codec.ReadFLVTag(reader); err == nil {
			switch t {
			case codec.FLV_TAG_TYPE_AUDIO:
				at.PushByteStream(timestamp+lastDisconnect, payload)
			case codec.FLV_TAG_TYPE_VIDEO:
				vt.PushByteStream(timestamp+lastDisconnect, payload)
			}
			if timestamp != 0 {
				elapse := time.Since(startTime)
				// 如果读取过快，导致时间戳超过真正流逝的时间，就需要睡眠，降低速度
				if elapse.Milliseconds() < int64(timestamp) {
					time.Sleep(time.Millisecond*time.Duration(timestamp) - elapse)
				}
			}
			lastTime = timestamp
		} else {
			return
		}
	}
}
func PullStream(streamPath, url string) error {
	stream := Stream{
		Type:       "HDL Pull",
		StreamPath: streamPath,
	}
	at := stream.NewAudioTrack(0)
	vt := stream.NewVideoTrack(0)
	if strings.HasPrefix(url, "http") {
		if res, err := http.Get(url); err == nil {
			if stream.Publish() {
				go func() {
					lastTs := pull(at, vt, res.Body, 0)
					if config.Reconnect {
						for stream.Err() == nil {
							time.Sleep(time.Second * 5)
							lastTs = pull(at, vt, res.Body, lastTs)
						}
					} else {
						stream.Close()
					}
				}()
			} else {
				return errors.New("Bad Name")
			}
		} else {
			return err
		}
	} else {
		stream.Type = "FLV File"
		if file, err := os.Open(url); err == nil {
			if stream.Publish() {
				go func() {
					lastTs := pull(at, vt, file, 0)
					if config.Reconnect {
						for stream.Err() == nil {
							file.Seek(0, io.SeekStart)
							lastTs = pull(at, vt, file, lastTs)
						}
					} else {
						file.Close()
						stream.Close()
					}
				}()
			} else {
				file.Close()
				return errors.New("Bad Name")
			}
		} else {
			return err
		}
	}
	return nil
}
