package hdl

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/codec"
	"m7s.live/engine/v4/util"
)

type HDLPuller struct {
	Publisher
	Puller
	absTS uint32 //绝对时间戳
	buf   util.Buffer
	pool  util.BytesPool
}

func NewHDLPuller() *HDLPuller {
	return &HDLPuller{
		buf:  util.Buffer(make([]byte, len(codec.FLVHeader))),
		pool: make(util.BytesPool, 17),
	}
}

func (puller *HDLPuller) Connect() (err error) {
	HDLPlugin.Info("connect", zap.String("remoteURL", puller.RemoteURL))
	if strings.HasPrefix(puller.RemoteURL, "http") {
		var res *http.Response
		client := http.DefaultClient
		if puller.Puller.Config.Proxy != "" {
			proxy, err := url.Parse(puller.Puller.Config.Proxy)
			if err != nil {
				return err
			}
			transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
			client = &http.Client{Transport: transport}
		}
		if res, err = client.Get(puller.RemoteURL); err == nil {
			if res.StatusCode != http.StatusOK {
				return io.EOF
			}
			puller.SetIO(res.Body)
		}
	} else {
		var res *os.File
		if res, err = os.Open(puller.RemoteURL); err == nil {
			puller.SetIO(res)
		}
	}
	if err == nil {
		head := puller.buf.SubBuf(0, len(codec.FLVHeader))
		if _, err = io.ReadFull(puller, head); err == nil {
			if head[0] != 'F' || head[1] != 'L' || head[2] != 'V' {
				err = codec.ErrInvalidFLV
			} else {
				configCopy := hdlConfig.GetPublishConfig()
				configCopy.PubAudio = head[4]&0x04 != 0
				configCopy.PubVideo = head[4]&0x01 != 0
				puller.Config = &configCopy
			}
		}
	}
	if err != nil {
		HDLPlugin.Error("connect", zap.Error(err))
	}
	return
}

func (puller *HDLPuller) Pull() (err error) {
	puller.buf.Reset()
	var startTs uint32
	for offsetTs := puller.absTS; err == nil && puller.Err() == nil; _, err = io.ReadFull(puller, puller.buf[:4]) {
		tmp := puller.buf.SubBuf(0, 11)
		_, err = io.ReadFull(puller, tmp)
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
		var frame util.BLL
		mem := puller.pool.Get(int(dataSize))
		frame.Push(mem)
		_, err = io.ReadFull(puller, mem.Value)
		if err != nil {
			return
		}
		puller.absTS = offsetTs + (timestamp - startTs)
		// fmt.Println(t, offsetTs, timestamp, startTs, puller.absTS)
		switch t {
		case codec.FLV_TAG_TYPE_AUDIO:
			puller.WriteAVCCAudio(puller.absTS, &frame, puller.pool)
		case codec.FLV_TAG_TYPE_VIDEO:
			puller.WriteAVCCVideo(puller.absTS, &frame, puller.pool)
		case codec.FLV_TAG_TYPE_SCRIPT:
			puller.Info("script", zap.ByteString("data", mem.Value))
			frame.Recycle()
		}
	}
	return
}
