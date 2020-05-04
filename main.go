package hdl

import (
	"log"
	"net/http"
	"strings"

	. "github.com/Monibuca/engine/v2"
	"github.com/Monibuca/engine/v2/avformat"
	. "github.com/logrusorgru/aurora"
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
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Content-Type", "video/x-flv")
		w.Write(avformat.FLVHeader)
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
