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
	sign := r.URL.Query().Get("sign")
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
	sub := Subscriber{Sign: sign, ID: r.RemoteAddr, Type: "FLV"}
	if err := sub.Subscribe(stringPath); err == nil {
		var buffer bytes.Buffer
		if _, err := amf.WriteString(&buffer, "onMetaData"); err != nil {
			return err
		}
		metaData := amf.Object{
			"MetaDataCreator": "m7s",
			"hasVideo":        sub.OriginVideoTrack != nil,
			"hasAudio":        sub.OriginAudioTrack != nil,
			"hasMatadata":     true,
			"canSeekToEnd":    false,
			"duration":        0,
			"hasKeyFrames":    0,
			"framerate":       0,
			"videodatarate":   0,
			"filesize":        0,
		}
		if sub.OriginVideoTrack != nil {
			metaData["videocodecid"] = sub.OriginVideoTrack.CodecID
			metaData["width"] = sub.OriginVideoTrack.SPSInfo.Width
			metaData["height"] = sub.OriginVideoTrack.SPSInfo.Height
			sub.OnVideo = func(pack VideoPack) {
				payload := codec.Nalu2RTMPTag(pack.Payload)
				defer utils.RecycleSlice(payload)
				codec.WriteFLVTag(w, codec.FLV_TAG_TYPE_VIDEO, pack.Timestamp, payload)
			}
		}
		if sub.OriginAudioTrack != nil {
			metaData["audiocodecid"] = int(sub.OriginAudioTrack.SoundFormat)
			metaData["audiosamplerate"] = sub.OriginAudioTrack.SoundRate
			metaData["audiosamplesize"] = int(sub.OriginAudioTrack.SoundSize)
			metaData["stereo"] = sub.OriginAudioTrack.SoundType == 1
			var aac byte
			if sub.OriginAudioTrack.SoundFormat == 10 {
				aac = sub.OriginAudioTrack.RtmpTag[0]
			}
			sub.OnAudio = func(pack AudioPack) {
				payload := codec.Audio2RTMPTag(aac, pack.Payload)
				defer utils.RecycleSlice(payload)
				codec.WriteFLVTag(w, codec.FLV_TAG_TYPE_AUDIO, pack.Timestamp, payload)
			}
		}
		if _, err := WriteEcmaArray(&buffer, metaData); err != nil {
			return err
		}
		codec.WriteFLVTag(w, codec.FLV_TAG_TYPE_SCRIPT, 0, buffer.Bytes())
		sub.Play(r.Context(), sub.OriginAudioTrack, sub.OriginVideoTrack)
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
