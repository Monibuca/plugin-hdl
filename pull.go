package hdl

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"strings"

	. "github.com/Monibuca/engine/v4"
	"github.com/Monibuca/engine/v4/codec"
	"github.com/Monibuca/engine/v4/util"
	"go.uber.org/zap"
)

func (puller *HDLPuller) pull() {
	puller.ReConnectCount++
	head := util.Buffer(make([]byte, len(codec.FLVHeader)))
	reader := bufio.NewReader(puller)
	_, err := io.ReadFull(reader, head)
	if err != nil {
		return
	}
	head.Reset()
	var startTs uint32
	for offsetTs := puller.absTS; err == nil && puller.Err() == nil; _, err = io.ReadFull(reader, head[:4]) {
		tmp := head.SubBuf(0, 11)
		_, err = io.ReadFull(reader, tmp)
		if err != nil {
			return
		}
		t := tmp.ReadByte()
		dataSize := tmp.ReadUint24()
		timestamp := tmp.ReadUint24() | uint32(tmp.ReadByte())<<24
		if startTs == 0 {
			startTs = timestamp
		}
		tmp.ReadUint24()
		payload := make([]byte, dataSize)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			return
		}
		puller.absTS = offsetTs + (timestamp - startTs)
		switch t {
		case codec.FLV_TAG_TYPE_AUDIO:
			puller.AudioTrack.WriteAVCC(puller.absTS, payload)
		case codec.FLV_TAG_TYPE_VIDEO:
			puller.VideoTrack.WriteAVCC(puller.absTS, payload)
		}
	}
}

type HDLPuller struct {
	Publisher
	Puller
	absTS uint32 //绝对时间戳
}

func (puller *HDLPuller) OnEvent(event any) {
	switch v := event.(type) {
	case PullEvent:
		if v > 0 {
			go func(count PullEvent) {
				puller.pull() //阻塞拉流
				// 如果流没有被关闭，则重连，重拉
				if !puller.Stream.IsClosed() {
					puller.OnEvent(count)
				}
			}(v + 1)
		} else {
			// TODO: 发布失败重新发布
			if plugin.Publish(puller.StreamPath, puller) == nil {
				if strings.HasPrefix(puller.RemoteURL, "http") {
					if res, err := http.Get(puller.RemoteURL); err == nil {
						puller.Reader = res.Body
						puller.Closer = res.Body
					} else {
						puller.Error(puller.RemoteURL, zap.Error(err))
						return
					}
				} else {
					if res, err := os.Open(puller.RemoteURL); err == nil {
						puller.Reader = res
						puller.Closer = res
					} else {
						puller.Error(puller.RemoteURL, zap.Error(err))
						return
					}
				}
				// 注入context
				puller.OnEvent(Engine)
				puller.OnEvent(PullEvent(1))
			}
		}
	default:
		puller.Publisher.OnEvent(event)
	}
}

func (config *HDLConfig) PullStream(puller Puller) {
	client := &HDLPuller{
		Puller: puller,
	}
	client.OnEvent(PullEvent(0))
}
