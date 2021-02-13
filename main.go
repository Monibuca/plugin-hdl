package hdl

import (
	"bytes"
	"encoding/binary"
	"net/http"
	"regexp"

	. "github.com/Monibuca/engine/v2"
	"github.com/Monibuca/engine/v2/avformat"
	"github.com/Monibuca/utils"
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
		Type:   PLUGIN_SUBSCRIBER,
		Config: &config,
		Run:    run,
	})
}

func run() {
	if config.ListenAddr != "" || config.ListenAddrTLS != "" {
		Print(Green("HDL start at "), BrightBlue(config.ListenAddr), BrightBlue(config.ListenAddrTLS))
		utils.ListenAddrs(config.ListenAddr, config.ListenAddrTLS, config.CertFile, config.KeyFile, http.HandlerFunc(HDLHandler))
	} else {
		Print(Green("HDL start reuse gateway port"))
		http.HandleFunc("/hdl/", HDLHandler)
	}
}

func HDLHandler(w http.ResponseWriter, r *http.Request) {
	sign := r.URL.Query().Get("sign")
	if err := AuthHooks.Trigger(sign); err != nil {
		w.WriteHeader(403)
		return
	}
	parts := streamPathReg.FindStringSubmatch(r.RequestURI)
	stringPath := parts[3]
	if stringPath == "" {
		stringPath = parts[5]
	}
	//atomic.AddInt32(&hdlId, 1)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "video/x-flv")
	w.Write(avformat.FLVHeader)
	p := Subscriber{
		Sign: sign,
		OnData: func(packet *avformat.SendPacket) error {
			return avformat.WriteFLVTag(w, packet)
		},
		MetaData: func(stream *Stream) error {
			var metadata avformat.SendPacket
			metadata.AVPacket = new(avformat.AVPacket)
			metadata.Type = avformat.FLV_TAG_TYPE_SCRIPT
			var buffer bytes.Buffer
			if _, err := amf.WriteString(&buffer, "onMetaData"); err != nil {
				return err
			}

			if _, err := WriteEcmaArray(&buffer, amf.Object{
				"MetaDataCreator": "monibuca",
				"hasVideo":        true,
				"hasAudio":        stream.AudioInfo.SoundFormat > 0,
				"hasMatadata":     true,
				"canSeekToEnd":    false,
				"duration":        0,
				"hasKeyFrames":    0,
				"videocodecid":    int(stream.VideoInfo.CodecID),
				"framerate":       0,
				"videodatarate":   0,
				"audiocodecid":    int(stream.AudioInfo.SoundFormat),
				"filesize":        0,
				"width":           stream.VideoInfo.SPSInfo.Width,
				"height":          stream.VideoInfo.SPSInfo.Height,
				"audiosamplerate": stream.AudioInfo.SoundRate,
				"audiosamplesize": int(stream.AudioInfo.SoundSize),
				"stereo":          stream.AudioInfo.SoundType == 1,
			}); err != nil {
				return err
			}
			metadata.Payload = buffer.Bytes()
			return avformat.WriteFLVTag(w, &metadata)
		},
		SubscriberInfo: SubscriberInfo{
			ID: r.RemoteAddr, Type: "FLV",
		},
	}
	p.SubscribeWithContext(stringPath, r.Context())
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
