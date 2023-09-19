package main

import (
	"os"
	"strconv"
	"time"
	"unsafe"

	"github.com/pion/rtp"
)

//1 Создать контекст (контейнер файл mp4)
// 2 угадываем otputformat
// 3 добавляем стрим
// 4 открываем файл
// 5 записываем хедеры
// 6 декодируем nalus в RGB фреймы
// 7 кодируем фреймы в пакеты
// 8 записываем в контекст

// #cgo pkg-config: libavcodec libavutil libswscale libavformat
// #include <libavcodec/avcodec.h>
// #include <libavutil/imgutils.h>
// #include <libswscale/swscale.h>
// #include <libavformat/avformat.h>
// #include <libavformat/avio.h>
import "C"

type h265RTPVideoWriter struct {
	output_format_context *C.AVFormatContext
	codecCtx              *C.AVCodecContext
	srcFrame              *C.AVFrame
	swsCtx                *C.struct_SwsContext
	dstFrame              *C.AVFrame
	dstFramePtr           []uint8
	enCodecCtx            *C.AVCodecContext
	packet_nb             int
	now                   int64
}

func NewH265RTPVideo() *h265RTPVideoWriter {
	file_name := (C.CString)(strconv.FormatInt(time.Now().UnixMilli(), 10) + ".mp4")
	var output_format_context *C.AVFormatContext

	if C.avformat_alloc_output_context2(&output_format_context, nil, nil, file_name) < 0 {
		panic("avformat_alloc_output_context2")
	}

	encoder := C.avcodec_find_encoder(C.AV_CODEC_ID_H265)
	if encoder == nil {
		panic("avcodec_find_encoder")
	}

	encoderCtx := C.avcodec_alloc_context3(encoder)
	if encoderCtx == nil {
		panic("avcodec_alloc_context3")
	}

	encoderCtx.time_base = C.AVRational{num: 1, den: 90000}
	encoderCtx.pix_fmt = C.AV_PIX_FMT_YUV420P
	encoderCtx.width = 3840
	encoderCtx.height = 2160
	encoderCtx.framerate = C.AVRational{num: 25, den: 1}
	encoderCtx.codec_type = C.AVMEDIA_TYPE_VIDEO
	// timebase, timescale, fps
	/*
		pts_time = pts * time_base
		frame=0, pts=0, pts_time = 0
	*/
	video_stream := C.avformat_new_stream(output_format_context, encoder)
	if video_stream == nil {
		panic("avformat_new_stream")
	}

	if (output_format_context.oformat.flags & C.AV_CODEC_FLAG_GLOBAL_HEADER) != 0 {
		encoderCtx.flags = encoderCtx.flags | C.AV_CODEC_FLAG_GLOBAL_HEADER
	}

	if C.avcodec_open2(encoderCtx, encoder, nil) < 0 {
		panic("avcodec_open2")
	}

	if C.avcodec_parameters_from_context(video_stream.codecpar, encoderCtx) < 0 {
		panic("avcodec_parameters_from_context")
	}

	if C.avio_open(&output_format_context.pb, file_name, C.AVIO_FLAG_WRITE) < 0 {
		panic("avio_open2")
	}

	if C.avformat_write_header(output_format_context, nil) < 0 {
		panic("avformat_write_header")
	}

	codec := C.avcodec_find_decoder(C.AV_CODEC_ID_H265)
	if codec == nil {
		panic("avcodec_find_decoder")
	}

	codecCtx := C.avcodec_alloc_context3(codec)
	if codecCtx == nil {
		panic("avcodec_alloc_context3")
	}

	res := C.avcodec_open2(codecCtx, codec, nil)
	if res < 0 {
		C.avcodec_close(codecCtx)
		panic("avcodec_open2")
	}

	srcFrame := C.av_frame_alloc()
	if srcFrame == nil {
		C.avcodec_close(codecCtx)
		panic("av_frame_alloc")
	}

	return &h265RTPVideoWriter{
		output_format_context: output_format_context,
		codecCtx:              codecCtx,
		srcFrame:              srcFrame,
		enCodecCtx:            encoderCtx,
		packet_nb:             0,
		now:                   time.Now().UnixMilli(),
	}
}

func (writer *h265RTPVideoWriter) Close() {
	C.av_write_trailer(writer.output_format_context)
}

func (writer *h265RTPVideoWriter) ProcessRtpPacketPayload(packet *rtp.Packet) {

}

func (writer *h265RTPVideoWriter) WriteNalu(nalu []byte, pts float64) error {
	nalu = append([]uint8{0x00, 0x00, 0x00, 0x01}, []uint8(nalu)...)

	// send NALU to decoder
	var avPacket C.AVPacket
	avPacket.data = (*C.uint8_t)(C.CBytes(nalu))
	defer C.free(unsafe.Pointer(avPacket.data))
	avPacket.size = C.int(len(nalu))

	res := C.avcodec_send_packet(writer.codecCtx, &avPacket)
	if res < 0 {
		return nil
	}

	// receive frame if available
	res = C.avcodec_receive_frame(writer.codecCtx, writer.srcFrame)
	if res < 0 {
		return nil
	}

	// ====================ENCODE=====================
	output_packet := C.av_packet_alloc()
	res = C.avcodec_send_frame(writer.enCodecCtx, writer.srcFrame)
	for res >= 0 {
		res = C.avcodec_receive_packet(writer.enCodecCtx, output_packet)
		if res < 0 {
			break
		}
		if writer.packet_nb == 0 {
			output_packet.pts = 0
			output_packet.dts = 0
			output_packet.duration = 0
		} else {
			frame_duration := writer.enCodecCtx.time_base.den / writer.enCodecCtx.framerate.num
			frame_time := C.longlong(writer.packet_nb * int(frame_duration))
			pts := frame_time / C.longlong(writer.enCodecCtx.time_base.num)

			output_packet.pts = pts
			output_packet.dts = pts
			output_packet.duration = C.longlong(frame_duration)
		}
		C.av_interleaved_write_frame(writer.output_format_context, output_packet)
		writer.packet_nb += 1
		if time.Now().UnixMilli()-writer.now > 40*1000 {
			C.av_write_trailer(writer.output_format_context)
			os.Exit(1)
		}
	}
	C.av_packet_unref(output_packet)
	//=======================ENCODE=====================

	return nil
}
