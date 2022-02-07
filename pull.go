package hdl

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	. "github.com/Monibuca/engine/v4"
	"github.com/Monibuca/engine/v4/codec"
	"github.com/Monibuca/engine/v4/track"
)

func (puller *HDLPuller) pull() {
	head := make([]byte, len(codec.FLVHeader))
	io.ReadFull(puller, head)
	startTime := time.Now()
	for {
		if t, timestamp, payload, err := codec.ReadFLVTag(puller); err == nil {
			switch t {
			case codec.FLV_TAG_TYPE_AUDIO:
				puller.at.WriteAVCC(timestamp+puller.lastTs, payload)
			case codec.FLV_TAG_TYPE_VIDEO:
				puller.vt.WriteAVCC(timestamp+puller.lastTs, payload)
			}
			if timestamp != 0 {
				elapse := time.Since(startTime)
				// 如果读取过快，导致时间戳超过真正流逝的时间，就需要睡眠，降低速度
				if elapse.Milliseconds() < int64(timestamp) {
					time.Sleep(time.Millisecond*time.Duration(timestamp) - elapse)
				}
			}
			puller.lastTs = timestamp
		} else {
			puller.UnPublish()
			return
		}
	}
}

type HDLPuller struct {
	Publisher
	lastTs uint32 //断线前的时间戳
	at     *track.UnknowAudio
	vt     *track.UnknowVideo
	io.ReadCloser
}

func (puller *HDLPuller) Close() {

}

func (puller *HDLPuller) OnStateChange(old StreamState, n StreamState) bool {
	switch n {
	case STATE_PUBLISHING:
		puller.at = puller.NewAudioTrack()
		puller.vt = puller.NewVideoTrack()
		if puller.Type == "HDL Pull" {
			if res, err := http.Get(puller.String()); err == nil {
				puller.ReadCloser = res.Body
			} else {
				return false
			}
		} else {
			if file, err := os.Open(puller.String()); err == nil {
				file.Seek(int64(len(codec.FLVHeader)), io.SeekStart)
				puller.ReadCloser = file
			} else {
				file.Close()
				return false
			}
		}
		go puller.pull()
	case STATE_WAITPUBLISH:
		if config.AutoReconnect {
			if puller.Type == "HDL Pull" {
				if res, err := http.Get(puller.String()); err == nil {
					puller.ReadCloser = res.Body
				} else {
					return true
				}
			} else {
				if file, err := os.Open(puller.String()); err == nil {
					file.Seek(int64(len(codec.FLVHeader)), io.SeekStart)
					puller.ReadCloser = file
				} else {
					file.Close()
					return true
				}
				go puller.pull()
			}
		}
	}
	return true
}

func PullStream(streamPath, address string) (err error) {
	puller := &HDLPuller{}
	puller.PullURL, err = url.Parse(address)
	if err != nil {
		return
	}
	puller.Config = config.PublishConfig
	if strings.HasPrefix(puller.Scheme, "http") {
		puller.Type = "HDL Pull"
		puller.Publish(streamPath, puller)
	} else {
		puller.Type = "FLV File"
		puller.Publish(streamPath, puller)
	}
	return nil
}
