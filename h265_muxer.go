package main

import (
	"fmt"
	"strconv"
	"time"
	"unsafe"
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
}

func NewH265RTPVideo() *h265RTPVideoWriter {
	file_name := (C.CString)(strconv.FormatInt(time.Now().UnixMilli(), 10) + ".mp4")
	var output_format_context *C.AVFormatContext
	// output_format := C.av_guess_format(nil, file_name, nil)
	// if output_format == nil {
	// 	panic("av_guess_format")
	// }
	// output_format.video_codec = C.AV_CODEC_ID_H265

	if C.avformat_alloc_output_context2(&output_format_context, nil, nil, file_name) < 0 {
		panic("avformat_alloc_output_context2")
	}
	// output_format_context.oformat = output_format

	encoder := C.avcodec_find_encoder(C.AV_CODEC_ID_H265)
	if encoder == nil {
		panic("avcodec_find_encoder")
	}

	encoderCtx := C.avcodec_alloc_context3(encoder)
	if encoderCtx == nil {
		panic("avcodec_alloc_context3")
	}

	encoderCtx.time_base = C.AVRational{num: 1, den: 25}
	encoderCtx.pix_fmt = C.AV_PIX_FMT_YUV420P
	encoderCtx.width = 1920
	encoderCtx.height = 1080
	encoderCtx.framerate = C.AVRational{num: 25, den: 1}
	encoderCtx.codec_type = C.AVMEDIA_TYPE_VIDEO

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
	}
}

func (writer *h265RTPVideoWriter) Close() {
	C.av_write_trailer(writer.output_format_context)
	fmt.Println("123123123")
}

func (writer *h265RTPVideoWriter) WriteNalu(nalu []byte) error {
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
	output_packet := C.av_packet_alloc()
	res = C.avcodec_send_frame(writer.enCodecCtx, writer.srcFrame)
	for res >= 0 {
		res = C.avcodec_receive_packet(writer.enCodecCtx, output_packet)
		if res < 0 {
			return nil
		}
		fmt.Println("сча запишет:", output_packet.size)

	}
	C.av_packet_unref(output_packet)
	// output_packet->duration = enc_video_avs->time_base.den / enc_video_avs->time_base.num /
	//                             dec_video_avs->avg_frame_rate.num * dec_video_avs->avg_frame_rate.den

	return nil
}
