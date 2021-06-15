package hdl

import (
	"bytes"
	"encoding/binary"
	"net/http"
	"regexp"

	. "github.com/Monibuca/engine/v3"
	"github.com/Monibuca/utils/v3"
	"github.com/Monibuca/utils/v3/codec"
	. "github.com/logrusorgru/aurora"
	amf "github.com/zhangpeihao/goamf"
)

var config struct {
	ListenAddr    string
	ListenAddrTLS string
	CertFile      string
	KeyFile       string
}
var streamPathReg = regexp.MustCompile("/(hdl/)?((.+)(\\.flv)|(.+))")

func init() {
	InstallPlugin(&PluginConfig{
		Name:   "HDL",
		Config: &config,
		Run:    run,
	})
}

func run() {
	if config.ListenAddr != "" || config.ListenAddrTLS != "" {
		utils.Print(Green("HDL start at "), BrightBlue(config.ListenAddr), BrightBlue(config.ListenAddrTLS))
		utils.ListenAddrs(config.ListenAddr, config.ListenAddrTLS, config.CertFile, config.KeyFile, http.HandlerFunc(HDLHandler))
	} else {
		utils.Print(Green("HDL start reuse gateway port"))
		http.HandleFunc("/hdl/", HDLHandler)
	}
}

func HDLHandler(w http.ResponseWriter, r *http.Request) {
	// if err := AuthHooks.Trigger(sign); err != nil {
	// 	w.WriteHeader(403)
	// 	return
	// }
	utils.CORS(w, r)
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
	w.Write(codec.FLVHeader)
	sub := Subscriber{ByteStreamFormat: true, ID: r.RemoteAddr, Type: "FLV", Ctx2: r.Context()}
	if err := sub.Subscribe(stringPath); err == nil {
		vt, at := sub.WaitVideoTrack(), sub.WaitAudioTrack()
		var buffer bytes.Buffer
		if _, err := amf.WriteString(&buffer, "onMetaData"); err != nil {
			return
		}
		metaData := amf.Object{
			"MetaDataCreator": "m7s",
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
		if vt != nil {
			metaData["videocodecid"] = int(vt.CodecID)
			metaData["width"] = vt.SPSInfo.Width
			metaData["height"] = vt.SPSInfo.Height
			codec.WriteFLVTag(w, codec.FLV_TAG_TYPE_VIDEO, 0, vt.ExtraData.Payload)
			sub.OnVideo = func(pack VideoPack) {
				codec.WriteFLVTag(w, codec.FLV_TAG_TYPE_VIDEO, pack.Timestamp, pack.Payload)
			}
		}
		if at != nil {
			metaData["audiocodecid"] = int(at.CodecID)
			metaData["audiosamplerate"] = at.SoundRate
			metaData["audiosamplesize"] = int(at.SoundSize)
			metaData["stereo"] = at.Channels == 2
			if at.CodecID == 10 {
				codec.WriteFLVTag(w, codec.FLV_TAG_TYPE_AUDIO, 0, at.ExtraData)
			}
			sub.OnAudio = func(pack AudioPack) {
				codec.WriteFLVTag(w, codec.FLV_TAG_TYPE_AUDIO, pack.Timestamp, pack.Payload)
			}
		}
		if _, err := WriteEcmaArray(&buffer, metaData); err != nil {
			return
		}
		codec.WriteFLVTag(w, codec.FLV_TAG_TYPE_SCRIPT, 0, buffer.Bytes())
		sub.Play(at, vt)
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
