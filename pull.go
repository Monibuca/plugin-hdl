package hdl

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"strings"

	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/codec"
	"m7s.live/engine/v4/util"
)

func (puller *HDLPuller) Connect() (err error) {
	HDLPlugin.Info("connect", zap.String("remoteURL", puller.RemoteURL))
	if strings.HasPrefix(puller.RemoteURL, "http") {
		var res *http.Response
		if res, err = http.Get(puller.RemoteURL); err == nil {
			puller.SetIO(res.Body)
		}
	} else {
		var res *os.File
		if res, err = os.Open(puller.RemoteURL); err == nil {
			puller.SetIO(res)
		}
	}
	if err != nil {
		puller.Error("connect", zap.Error(err))
	}
	return
}
func (puller *HDLPuller) Pull() {
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
			puller.WriteAVCCAudio(puller.absTS, payload)
		case codec.FLV_TAG_TYPE_VIDEO:
			puller.WriteAVCCVideo(puller.absTS, payload)
		}
	}
}

type HDLPuller struct {
	Publisher
	Puller
	absTS uint32 //绝对时间戳
}
