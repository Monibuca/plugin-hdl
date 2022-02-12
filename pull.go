package hdl

import (
	"bufio"
	"io"
	"net/http"
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
	Puller
	absTS uint32 //绝对时间戳
	at    *track.UnknowAudio
	vt    *track.UnknowVideo
}
// 用于发布FLV文件
type FLVFile struct {
	HDLPuller
}

func (puller *FLVFile) Pull(count int) {
	if count == 0 {
		puller.at = puller.NewAudioTrack()
		puller.vt = puller.NewVideoTrack()
	}
	if file, err := os.Open(puller.RemoteURL); err == nil {
		puller.Reader = file
		puller.Closer = file
	} else {
		file.Close()
		return
	}
	puller.pull()
}

func (puller *HDLPuller) Pull(count int) {
	if count == 0 {
		puller.at = puller.NewAudioTrack()
		puller.vt = puller.NewVideoTrack()
	}
	if res, err := http.Get(puller.RemoteURL); err == nil {
		puller.Reader = res.Body
		puller.Closer = res.Body
	} else {
		puller.Error(err)
		return
	}
	puller.pull()
}

func (config *HDLConfig) PullStream(streamPath string, puller Puller) bool {
	var puber IPublisher = &HDLPuller{Puller: puller}
	if !strings.HasPrefix(puller.RemoteURL, "http") {
		puber = &FLVFile{HDLPuller: *puber.(*HDLPuller)}
	}
	return puber.Publish(streamPath, puber, Config.Publish)
}
