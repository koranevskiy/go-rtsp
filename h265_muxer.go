package main

import (
	"strconv"
	"time"
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

	encoderCtx.time_base = C.AVRational{num: 1, den: 25}
	encoderCtx.pix_fmt = C.AV_PIX_FMT_YUV420P
	encoderCtx.width = 1920
	encoderCtx.height = 1080

	decoder := C.avcodec_find_decoder(C.AV_CODEC_ID_H265)
	if decoder == nil {
		panic("avcodec_find_decoder")
	}

	video_stream := C.avformat_new_stream(output_format_context, encoder)
	if video_stream == nil {
		panic("avformat_new_stream")
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

	return &h265RTPVideoWriter{}
}

func (writer *h265RTPVideoWriter) Close() {

}

func (writer *h265RTPVideoWriter) WriteNalu(nalu []byte) error {

	return nil
}
