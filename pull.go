package hdl

import (
	"bufio"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	. "github.com/Monibuca/engine/v4"
	"github.com/Monibuca/engine/v4/codec"
	"github.com/Monibuca/engine/v4/track"
	"github.com/Monibuca/engine/v4/util"
)

func (puller *HDLPuller) pull() {
	head := util.Buffer(make([]byte, len(codec.FLVHeader)))
	reader := bufio.NewReader(puller)
	_, err := io.ReadFull(reader, head)
	if err != nil {
		return
	}
	head.Reset()
	var startTime time.Time
	var startTs uint32
	defer puller.UnPublish()
	for offsetTs := puller.absTS; err == nil; _, err = io.ReadFull(reader, head[:4]) {
		tmp := head.SubBuf(0, 11)
		_, err = io.ReadFull(reader, tmp)
		if err != nil {
			return
		}
		t := tmp.ReadByte()
		dataSize := tmp.ReadUint24()
		timestamp := tmp.ReadUint24() | uint32(tmp.ReadByte())<<24
		tmp.ReadUint24()
		payload := make([]byte, dataSize)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			return
		}
		timestamp -= startTs // 相对时间戳
		puller.absTS = offsetTs + timestamp
		switch t {
		case codec.FLV_TAG_TYPE_AUDIO:
			puller.at.WriteAVCC(puller.absTS, payload)
		case codec.FLV_TAG_TYPE_VIDEO:
			puller.vt.WriteAVCC(puller.absTS, payload)
		}
		if timestamp != 0 {
			if startTs == 0 {
				startTs = timestamp
				startTime = time.Now()
			} else if fast := time.Duration(timestamp)*time.Millisecond - time.Since(startTime); fast > 0 {
				// 如果读取过快，导致时间戳超过真正流逝的时间，就需要睡眠，降低速度
				time.Sleep(fast)
			}
		}
	}
}

type HDLPuller struct {
	Publisher
	absTS uint32 //绝对时间戳
	at    *track.UnknowAudio
	vt    *track.UnknowVideo
	io.ReadCloser
}

func (puller *HDLPuller) Close() {
	puller.ReadCloser.Close()
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
