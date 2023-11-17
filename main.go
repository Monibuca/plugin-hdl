package hdl // import "m7s.live/plugin/hdl/v4"

import (
	"net"
	"net/http"
	"strings"
	"time"

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

func pull(streamPath, url string) {
	if err := HDLPlugin.Pull(streamPath, url, NewHDLPuller(), 0); err != nil {
		HDLPlugin.Error("pull", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
	}
}
func (c *HDLConfig) OnEvent(event any) {
	switch v := event.(type) {
	case FirstConfig:
		for streamPath, url := range c.PullOnStart {
			pull(streamPath, url)
		}
	case InvitePublish: //按需拉流
	if remoteURL := c.CheckPullOnSub(v.Target); remoteURL != "" {
			pull(v.Target, remoteURL)
		}
	}
}

func str2number(s string) int {
	switch s {
	case "1":
		return 1
	case "2":
		return 2
	default:
		return 0
	}
}

func (c *HDLConfig) API_Pull(rw http.ResponseWriter, r *http.Request) {
	err := HDLPlugin.Pull(r.URL.Query().Get("streamPath"), r.URL.Query().Get("target"), NewHDLPuller(), str2number(r.URL.Query().Get("save")))
	if err != nil {
		util.ReturnError(util.APIErrorPublish, err.Error(), rw, r)
	} else {
		util.ReturnOK(rw, r)
	}
}

func (*HDLConfig) API_List(rw http.ResponseWriter, r *http.Request) {
	util.ReturnFetchValue(FilterStreams[*HDLPuller], rw, r)
}

// 确保HDLConfig实现了PullPlugin接口
var hdlConfig = new(HDLConfig)
var HDLPlugin = InstallPlugin(hdlConfig)

type HDLSubscriber struct {
	Subscriber
}

func (sub *HDLSubscriber) OnEvent(event any) {
	switch v := event.(type) {
	case FLVFrame:
		// t := time.Now()
		// s := util.SizeOfBuffers(v)
		if hdlConfig.WriteTimeout > 0 {
			if conn, ok := sub.Writer.(net.Conn); ok {
				conn.SetWriteDeadline(time.Now().Add(hdlConfig.WriteTimeout))
			}
		}
		if _, err := v.WriteTo(sub); err != nil {
			sub.Stop(zap.Error(err))
			// } else {
			// println(time.Since(t)/time.Millisecond, s)
		}
	default:
		sub.Subscriber.OnEvent(event)
	}
}

func (sub *HDLSubscriber) WriteFlvHeader() {
	at, vt := sub.Audio, sub.Video
	hasAudio, hasVideo := at != nil, vt != nil
	var amf util.AMF
	amf.Marshal("onMetaData")
	metaData := util.EcmaArray{
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
	amf.Marshal(metaData)
	// 写入FLV头
	sub.Write([]byte{'F', 'L', 'V', 0x01, flags, 0, 0, 0, 9, 0, 0, 0, 0})
	codec.WriteFLVTag(sub, codec.FLV_TAG_TYPE_SCRIPT, 0, amf.Buffer)
}

func (c *HDLConfig) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	streamPath := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), ".flv")
	if r.URL.RawQuery != "" {
		streamPath += "?" + r.URL.RawQuery
	}
	sub := &HDLSubscriber{}
	sub.ID = r.RemoteAddr
	sub.SetParentCtx(r.Context())
	sub.SetIO(w)
	if err := HDLPlugin.Subscribe(streamPath, sub); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "video/x-flv")
		w.Header().Set("Transfer-Encoding", "identity")
		w.WriteHeader(http.StatusOK)
		if hijacker, ok := w.(http.Hijacker); ok && c.WriteTimeout > 0 {
			conn, _, _ := hijacker.Hijack()
			conn.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
			sub.SetIO(conn)
		} else {
			w.(http.Flusher).Flush()
		}
		sub.WriteFlvHeader()
		sub.PlayBlock(SUBTYPE_FLV)
	}
}
