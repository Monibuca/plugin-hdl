package hdl

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/http"
	"regexp"
	"time"

	. "github.com/Monibuca/engine/v4"
	"github.com/Monibuca/engine/v4/codec"
	"github.com/Monibuca/engine/v4/config"
	"github.com/Monibuca/engine/v4/util"
	. "github.com/logrusorgru/aurora"
	amf "github.com/zhangpeihao/goamf"
)

type HDLConfig struct {
	config.HTTP
	config.Publish
	config.Subscribe
	config.Pull
}

var streamPathReg = regexp.MustCompile(`/(hdl/)?((.+)(\.flv)|(.+))`)

func (config *HDLConfig) Update(override config.Config) {
	override.Unmarshal(config)
	if config.PullOnStart {
		for streamPath, url := range config.AutoPullList {
			if err := PullStream(streamPath, url); err != nil {
				util.Println(err)
			}
		}
	}
	if config.ListenAddr != "" || config.ListenAddrTLS != "" {
		util.Print(Green("HDL Listen at "), BrightBlue(config.ListenAddr), BrightBlue(config.ListenAddrTLS))
		config.Listen(plugin, config)
	}
}
func (config *HDLConfig) API_pull(rw http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("target")
	streamPath := r.URL.Query().Get("streamPath")
	save := r.URL.Query().Get("save")
	if err := PullStream(streamPath, targetURL); err == nil {
		if save == "1" {
			if config.AutoPullList == nil {
				config.AutoPullList = make(map[string]string)
			}
			config.AutoPullList[streamPath] = targetURL
			if err = plugin.Save(); err != nil {
				util.Println(err)
			}
		}
		rw.WriteHeader(200)
	} else {
		rw.WriteHeader(500)
	}
}

var hdlConfig = new(HDLConfig)
var plugin = InstallPlugin(hdlConfig)

func init() {
	plugin.HandleApi("/list", util.GetJsonHandler(FilterStreams[*HDLPuller], time.Second))
	plugin.HandleFunc("/", hdlConfig.ServeHTTP)
}

func (config *HDLConfig) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := streamPathReg.FindStringSubmatch(r.RequestURI)
	if len(parts) == 0 {
		w.WriteHeader(404)
		return
	}
	stringPath := parts[3]
	if stringPath == "" {
		stringPath = parts[5]
	}
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "video/x-flv")
	sub := Subscriber{ID: r.RemoteAddr, Type: "FLV"}
	if sub.Subscribe(stringPath, hdlConfig.Subscribe) {
		vt, at := sub.WaitVideoTrack(), sub.WaitAudioTrack()
		var buffer bytes.Buffer
		if _, err := amf.WriteString(&buffer, "onMetaData"); err != nil {
			return
		}
		metaData := amf.Object{
			"MetaDataCreator": "m7s" + Engine.Version,
			"hasVideo":        vt != nil,
			"hasAudio":        at != nil,
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
		if at != nil {
			flags |= (1 << 2)
		}
		if vt != nil {
			flags |= 1
		}
		w.Write([]byte{'F', 'L', 'V', 0x01, flags, 0, 0, 0, 9, 0, 0, 0, 0})
		codec.WriteFLVTag(w, codec.FLV_TAG_TYPE_SCRIPT, 0, net.Buffers{buffer.Bytes()})
		if vt != nil {
			metaData["videocodecid"] = int(vt.CodecID)
			metaData["width"] = vt.SPSInfo.Width
			metaData["height"] = vt.SPSInfo.Height
			vt.DecoderConfiguration.FLV.WriteTo(w)
			sub.OnVideo = func(frame *VideoFrame) error {
				frame.FLV.WriteTo(w)
				return r.Context().Err()
			}
		}
		if at != nil {
			metaData["audiocodecid"] = int(at.CodecID)
			metaData["audiosamplerate"] = at.SampleRate
			metaData["audiosamplesize"] = at.SampleSize
			metaData["stereo"] = at.Channels == 2
			if at.CodecID == 10 {
				at.DecoderConfiguration.FLV.WriteTo(w)
			}
			sub.OnAudio = func(frame *AudioFrame) error {
				frame.FLV.WriteTo(w)
				return r.Context().Err()
			}
		}
		sub.Play(at, vt)
	} else {
		w.WriteHeader(500)
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
