package hdl

import (
	"bytes"
	"encoding/binary"
	"log"
	"net/http"
	"strings"

	. "github.com/Monibuca/engine/v2"
	"github.com/Monibuca/engine/v2/avformat"
	. "github.com/logrusorgru/aurora"
	"github.com/zhangpeihao/goamf"
)

var config = new(ListenerConfig)

func init() {
	InstallPlugin(&PluginConfig{
		Name:   "HDL",
		Type:   PLUGIN_SUBSCRIBER,
		Config: config,
		Run:    run,
	})
}

func run() {
	Print(Green("HDL start at "), BrightBlue(config.ListenAddr))
	log.Fatal(http.ListenAndServe(config.ListenAddr, http.HandlerFunc(HDLHandler)))
}

func HDLHandler(w http.ResponseWriter, r *http.Request) {
	sign := r.URL.Query().Get("sign")
	if err := AuthHooks.Trigger(sign); err != nil {
		w.WriteHeader(403)
		return
	}
	stringPath := strings.TrimLeft(r.RequestURI, "/")
	if strings.HasSuffix(stringPath, ".flv") {
		stringPath = strings.TrimRight(stringPath, ".flv")
	}
	if s := FindStream(stringPath); s != nil {
		//atomic.AddInt32(&hdlId, 1)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Content-Type", "video/x-flv")
		w.Write(avformat.FLVHeader)
		var metadata avformat.SendPacket
		metadata.AVPacket = new(avformat.AVPacket)
		metadata.Type = avformat.FLV_TAG_TYPE_SCRIPT
		var buffer bytes.Buffer
		amf.WriteString(&buffer, "onMetaData")
		WriteEcmaArray(&buffer, amf.Object{
			"MetaDataCreator": "monibuca",
			"hasVideo":        true,
			"hasAudio":        true,
			"hasMatadata":     true,
			"canSeekToEnd":    false,
			"duration":        0,
			"hasKeyFrames":    0,
			"videocodecid":    int(s.VideoInfo.CodecID),
			"framerate":       0,
			"videodatarate":   0,
			"audiocodecid":    int(s.AudioInfo.SoundFormat),
			"filesize":        0,
			"width":           s.VideoInfo.SPSInfo.Width,
			"height":          s.VideoInfo.SPSInfo.Height,
			"audiosamplerate": s.AudioInfo.SoundRate,
			"audiosamplesize": int(s.AudioInfo.SoundSize),
			"stereo":          s.AudioInfo.SoundType == 1,
		})
		metadata.Payload = buffer.Bytes()
		avformat.WriteFLVTag(w, &metadata)
		p := Subscriber{
			Sign: sign,
			OnData: func(packet *avformat.SendPacket) error {
				return avformat.WriteFLVTag(w, packet)
			},
			SubscriberInfo: SubscriberInfo{
				ID: r.RemoteAddr, Type: "FLV",
			},
		}
		p.Subscribe(stringPath)
	} else {
		w.WriteHeader(404)
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
