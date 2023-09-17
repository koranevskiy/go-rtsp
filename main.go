package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph265"
	"github.com/bluenviron/gortsplib/v4/pkg/url"
	"github.com/pion/rtp"
)

const conStr = "rtsp://video:qG4RXkJ3d63t@10.10.17.29:554/cam/realmonitor?channel=1&subtype=0&unicast=true&proto=Onvif"

// 1 подключиться к rtsp и получать пакеты
// 2 функция при получении пакета
// 3 функция декодирования пакета
// 4 запись декодированного пакета в файл

func init() {
	os.MkdirAll("images", os.ModePerm)
}

func main() {
	// 1
	client := gortsplib.Client{}
	defer client.Close()
	cUrl, _ := url.Parse(conStr)
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
	//asd 12

	muxer := NewH265RTPVideo()
	defer muxer.Close()

	return

	if format.VPS != nil {
		frameDec.decode(format.VPS)
		muxer.WriteNalu(format.VPS)
	}
	if format.SPS != nil {
		frameDec.decode(format.SPS)
		muxer.WriteNalu(format.SPS)
	}
	if format.PPS != nil {
		frameDec.decode(format.PPS)
		muxer.WriteNalu(format.PPS)
	}

	// Setup request with ports - setup connection
	client.Setup(cUrl, mediaStream, 0, 0)
	rtpDecoder, e := format.CreateDecoder()
	if e != nil {
		fmt.Println("Can not create decoder")
		panic(err)
	}

	saveCount := 0
	client.OnPacketRTP(mediaStream, format, func(packet *rtp.Packet) {
		onPacketRecieved(packet, client, mediaStream, rtpDecoder, frameDec, saveCount, muxer)
	})
	client.Play(nil)
	client.Wait()

}

// 2
func onPacketRecieved(
	packet *rtp.Packet,
	client gortsplib.Client,
	mediaStream *description.Media,
	rtpDecoder *rtph265.Decoder,
	frameDec *h265Decoder, s int,
	muxer *h265RTPVideoWriter) {
	decodePacket(packet, client, mediaStream, rtpDecoder, frameDec, s, muxer)
	// fmt.Println(packet)
}

// 3 декодинг
func decodePacket(
	packet *rtp.Packet,
	client gortsplib.Client,
	mediaStream *description.Media,
	rtpDecoder *rtph265.Decoder,
	frameDec *h265Decoder,
	saveCount int,
	muxer *h265RTPVideoWriter) {
	_, ok := client.PacketPTS(mediaStream, packet) // вернет еще timestamp pts
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
		muxer.WriteNalu(nalu)
		img, err := frameDec.decode(nalu)
		if err != nil {
			panic(err)
		}

		// wait for a frame
		if img == nil {
			continue
		}
		saveToFile(img)
	}

}

func saveToFile(img image.Image) error {
	// create file
	fname := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10) + ".jpg"
	f, err := os.Create(filepath.Join("images", fname))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// convert to jpeg
	return jpeg.Encode(f, img, &jpeg.Options{
		Quality: 60,
	})
}
