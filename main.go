package hdl // import "m7s.live/plugin/hdl/v4"

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/http"
	"strings"
	"time"

	amf "github.com/zhangpeihao/goamf"
	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/codec"
	"m7s.live/engine/v4/config"
	"m7s.live/engine/v4/util"
)

type HDLConfig struct {
	config.HTTP
	config.Publish
	config.Subscribe
	config.Pull
}

func (c *HDLConfig) OnEvent(event any) {
	switch v := event.(type) {
	case FirstConfig:
		if c.PullOnStart {
			for streamPath, url := range c.PullList {
				if err := HDLPlugin.Pull(streamPath, url, new(HDLPuller), false); err != nil {
					HDLPlugin.Error("pull", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
				}
			}
		}
	case *Stream: //按需拉流
		if c.PullOnSubscribe {
			for streamPath, url := range c.PullList {
				if streamPath == v.Path {
					if err := HDLPlugin.Pull(streamPath, url, new(HDLPuller), false); err != nil {
						HDLPlugin.Error("pull", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
					}
					break
				}
			}
		}
	}
}

func (c *HDLConfig) API_Pull(rw http.ResponseWriter, r *http.Request) {
	err := HDLPlugin.Pull(r.URL.Query().Get("streamPath"), r.URL.Query().Get("target"), new(HDLPuller), r.URL.Query().Has("save"))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
	} else {
		rw.Write([]byte("ok"))
	}
}

func (*HDLConfig) API_List(rw http.ResponseWriter, r *http.Request) {
	util.ReturnJson(FilterStreams[*HDLPuller], time.Second, rw, r)
}

// 确保HDLConfig实现了PullPlugin接口

var HDLPlugin = InstallPlugin(new(HDLConfig))

type HDLSubscriber struct {
	Subscriber
}

func (sub *HDLSubscriber) OnEvent(event any) {
	switch v := event.(type) {
	case ISubscriber:
		at, vt := sub.Audio.Track, sub.Video.Track
		hasAudio, hasVideo := at != nil, vt != nil
		var buffer bytes.Buffer
		if _, err := amf.WriteString(&buffer, "onMetaData"); err != nil {
			return
		}
		metaData := amf.Object{
			"MetaDataCreator": "m7s" + Engine.Version,
			"hasVideo":        hasVideo,
			"hasAudio":        hasAudio,
			"hasMatadata":     true,
			"canSeekToEnd":    false,
			"duration":        0,
			"hasKeyFrames":    0,
			"framerate":       0,
			"videodatarate":   0,
			"filesize":        0,
		}
		if _, err := WriteEcmaArray(&buffer, metaData); err != nil {
			return
		}
		var flags byte
		if hasAudio {
			flags |= (1 << 2)
			metaData["audiocodecid"] = int(at.CodecID)
			metaData["audiosamplerate"] = at.SampleRate
			metaData["audiosamplesize"] = at.SampleSize
			metaData["stereo"] = at.Channels == 2
		}
		if hasVideo {
			flags |= 1
			metaData["videocodecid"] = int(vt.CodecID)
			metaData["width"] = vt.SPSInfo.Width
			metaData["height"] = vt.SPSInfo.Height
		}
		// 写入FLV头
		sub.Write([]byte{'F', 'L', 'V', 0x01, flags, 0, 0, 0, 9, 0, 0, 0, 0})
		codec.WriteFLVTag(sub, codec.FLV_TAG_TYPE_SCRIPT, 0, net.Buffers{buffer.Bytes()})
	case FLVFrame:
		if _, err := v.WriteTo(sub); err != nil {
			sub.Stop()
		}
	default:
		sub.Subscriber.OnEvent(event)
	}
}

func (*HDLConfig) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	streamPath := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/hdl/"), ".flv")
	if r.URL.RawQuery != "" {
		streamPath += "?" + r.URL.RawQuery
	}
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "video/x-flv")
	sub := &HDLSubscriber{}
	sub.ID = r.RemoteAddr
	sub.SetParentCtx(r.Context())
	sub.SetIO(w)
	if err := HDLPlugin.SubscribeBlock(streamPath, sub, SUBTYPE_FLV); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
func WriteEcmaArray(w amf.Writer, o amf.Object) (n int, err error) {
	n, err = amf.WriteMarker(w, amf.AMF0_ECMA_ARRAY_MARKER)
	if err != nil {
		return
	}
	length := int32(len(o))
	err = binary.Write(w, binary.BigEndian, &length)
	if err != nil {
		return
	}
	n += 4
	m := 0
	for name, value := range o {
		m, err = amf.WriteObjectName(w, name)
		if err != nil {
			return
		}
		n += m
		m, err = amf.WriteValue(w, value)
		if err != nil {
			return
		}
		n += m
	}
	m, err = amf.WriteObjectEndMarker(w)
	return n + m, err
}
