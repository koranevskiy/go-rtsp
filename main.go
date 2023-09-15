package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph265"
	"github.com/bluenviron/gortsplib/v4/pkg/url"
	"github.com/pion/rtp"
)

const conStr = "rtsp://video:qG4RXkJ3d63t@10.10.17.29:554/cam/realmonitor?channel=1&subtype=1&unicast=true&proto=Onvif"

// 1 подключиться к rtsp и получать пакеты
// 2 функция при получении пакета
// 3 функция декодирования пакета
// 4 запись декодированного пакета в файл

var file, _ = os.Create("file.h265")

func main() {
	defer file.Close()
	// 1
	client := gortsplib.Client{}
	defer client.Close()
	cUrl, _ := url.Parse(conStr)
	fmt.Println(cUrl.Host, cUrl.Scheme)
	// setup client fields
	err := client.Start(cUrl.Scheme, cUrl.Host)
	if err != nil {
		fmt.Println("Can not initialize connection")
		panic(err)
	}

	aboutSession, _, err := client.Describe(cUrl)
	if err != nil {
		fmt.Println("Can not describe session")
		panic(err)
	}
	var format *format.H265
	// if success initializes format
	mediaStream := aboutSession.FindFormat(&format)
	if mediaStream == nil {
		fmt.Println("There is no such media with h265 format")
		return
	}

	// setup H265 -> raw frames decoder
	frameDec, err := newH265Decoder()
	if err != nil {
		panic(err)
	}
	defer frameDec.close()

	// if VPS, SPS and PPS are present into the SDP, send them to the decoder
	if format.VPS != nil {
		frameDec.decode(format.VPS)
	}
	if format.SPS != nil {
		frameDec.decode(format.SPS)
	}
	if format.PPS != nil {
		frameDec.decode(format.PPS)
	}

	// Setup request with ports - setup connection
	client.Setup(cUrl, mediaStream, 0, 0)
	rtpDecoder, e := format.CreateDecoder()
	if e != nil {
		fmt.Println("Can not create decoder")
		panic(err)
	}
	client.OnPacketRTP(mediaStream, format, func(packet *rtp.Packet) {
		onPacketRecieved(packet, client, mediaStream, rtpDecoder, frameDec)
	})
	client.Play(nil)
	client.Wait()

}

// 2
func onPacketRecieved(packet *rtp.Packet, client gortsplib.Client, mediaStream *description.Media, rtpDecoder *rtph265.Decoder, frameDec *h265Decoder) {
	decodePacket(packet, client, mediaStream, rtpDecoder, frameDec)
	// fmt.Println(packet)
}

// 3 декодинг
func decodePacket(packet *rtp.Packet, client gortsplib.Client, mediaStream *description.Media, rtpDecoder *rtph265.Decoder, frameDec *h265Decoder) {
	pts, ok := client.PacketPTS(mediaStream, packet) // вернет еще timestamp pts
	// во тут вопросы есть
	if !ok {
		log.Printf("await for timestamp")
		return
	}

	// это уже набор NALU (инфа о всем кадре, нужно по этим nalus получить исходный растровый фрейм)
	accessU, err := rtpDecoder.Decode(packet)
	if err != nil {
		if err != rtph265.ErrNonStartingPacketAndNoPrevious && err != rtph265.ErrMorePacketsNeeded {
			log.Printf("ERR: %v", err)
		}
		return
	}
	for _, nalu := range accessU {
		// convert NALUs into RGBA frames
		img, err := frameDec.decode(nalu)
		if err != nil {
			panic(err)
		}

		// wait for a frame
		if img == nil {
			continue
		}

		log.Printf("decoded frame with PTS %v and size %v", pts, img.Bounds().Max)
	}

}
