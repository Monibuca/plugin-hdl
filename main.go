package hdl

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/http"
	"strings"
	"time"

	. "github.com/Monibuca/engine/v4"
	"github.com/Monibuca/engine/v4/codec"
	"github.com/Monibuca/engine/v4/config"
	"github.com/Monibuca/engine/v4/util"
	. "github.com/logrusorgru/aurora"
	amf "github.com/zhangpeihao/goamf"
	"go.uber.org/zap"
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
		if c.ListenAddr != "" || c.ListenAddrTLS != "" {
			plugin.Info(Green("HDL Server Start").String(), zap.String("ListenAddr", c.ListenAddr), zap.String("ListenAddrTLS", c.ListenAddrTLS))
			go c.Listen(plugin, c)
		} else {
			plugin.Info(Green("HDL start reuse engine port").String())
		}
	case PullerPromise:
		puller := v.Value
		client := &HDLPuller{
			Puller: puller,
		}
		err := client.connect()
		if err == nil {
			if err = plugin.Publish(puller.StreamPath, client); err == nil {
				v.Resolve(util.Null)
				break
			}
		}
		client.Error(puller.RemoteURL, zap.Error(err))
		v.Reject(err)
	}
}

func (c *HDLConfig) API_Pull(rw http.ResponseWriter, r *http.Request) {
	err := plugin.Pull(r.URL.Query().Get("streamPath"), r.URL.Query().Get("target"), r.URL.Query().Has("save"))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
	}
}

func (*HDLConfig) API_List(rw http.ResponseWriter, r *http.Request) {
	util.ReturnJson(FilterStreams[*HDLPuller], time.Second, rw, r)
}

// 确保HDLConfig实现了PullPlugin接口

var plugin = InstallPlugin(new(HDLConfig))

type HDLSubscriber struct {
	Subscriber
}

func (sub *HDLSubscriber) OnEvent(event any) {
	switch v := event.(type) {
	case HaveFLV:
		flvTag := v.GetFLV()
		flvTag.WriteTo(sub)
	default:
		sub.Subscriber.OnEvent(event)
	}
}

func (*HDLConfig) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	streamPath := strings.TrimPrefix(r.URL.Path, "/hls")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "video/x-flv")
	sub := &HDLSubscriber{}
	sub.ID = r.RemoteAddr
	sub.OnEvent(r.Context()) //注入父级Context
	sub.OnEvent(w)           //注入Writer
	if err := plugin.Subscribe(streamPath, sub); err == nil {
		at, vt := sub.AudioTrack, sub.VideoTrack
		hasVideo := at != nil
		hasAudio := vt != nil
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
		}
		if hasVideo {
			flags |= 1
		}
		w.Write([]byte{'F', 'L', 'V', 0x01, flags, 0, 0, 0, 9, 0, 0, 0, 0})
		if hasVideo {
			metaData["videocodecid"] = int(vt.CodecID)
			metaData["width"] = vt.SPSInfo.Width
			metaData["height"] = vt.SPSInfo.Height
		}
		if hasVideo {
			metaData["audiocodecid"] = int(at.CodecID)
			metaData["audiosamplerate"] = at.SampleRate
			metaData["audiosamplesize"] = at.SampleSize
			metaData["stereo"] = at.Channels == 2
		}
		codec.WriteFLVTag(w, codec.FLV_TAG_TYPE_SCRIPT, 0, net.Buffers{buffer.Bytes()})
		sub.PlayBlock(sub)
	} else {
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
