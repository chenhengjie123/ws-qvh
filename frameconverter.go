package main

import (
	"fmt"
	"unsafe"

	// "github.com/chenhengjie123/gmf"
	"github.com/chenhengjie123/ffmpeg-go/ffcommon"
	"github.com/chenhengjie123/ffmpeg-go/libavcodec"
	"github.com/chenhengjie123/ffmpeg-go/libavutil"
	"github.com/chenhengjie123/ffmpeg-go/libswscale"
	log "github.com/sirupsen/logrus"
	// "github.com/giorgisio/goav/swscale"
)

type FrameConverter struct {
	targetWidth   int
	targetHeight  int
	targetBitrate int
	originWidth   int
	originHeight  int
	decoder       *libavcodec.AVCodec
	decodeCtx     *libavcodec.AVCodecContext
	origFrame     *libavutil.AVFrame
	swsCtx        *libswscale.SwsContext
	scaledFrame   *libavutil.AVFrame
	encoder       *libavcodec.AVCodec
	encoderCtx    *libavcodec.AVCodecContext
}

func NewFrameConverter(width int, height int, bitrate int) *FrameConverter {

	// 初始化
	ffcommon.SetAvcodecPath("/usr/local/ffmpeg/lib/libavcodec.dylib")
	ffcommon.SetAvutilPath("/usr/local/ffmpeg/lib/libavutil.dylib")
	ffcommon.SetAvdevicePath("/usr/local/ffmpeg/lib/libavdevice.dylib")
	ffcommon.SetAvfilterPath("/usr/local/ffmpeg/lib/libavfilter.dylib")
	ffcommon.SetAvformatPath("/usr/local/ffmpeg/lib/libavformat.dylib")
	ffcommon.SetAvpostprocPath("/usr/local/ffmpeg/lib/libpostproc.dylib")
	ffcommon.SetAvswresamplePath("/usr/local/ffmpeg/lib/libswresample.dylib")
	ffcommon.SetAvswscalePath("/usr/local/ffmpeg/lib/libswscale.dylib")

	// Initialize the AVCodecContext and AVFrame
	decoder := libavcodec.AvcodecFindDecoder(libavcodec.AV_CODEC_ID_H264)
	if decoder == nil {
		fmt.Errorf("failed to find codec")
	}
	decodeCtx := decoder.AvcodecAllocContext3()
	if decodeCtx == nil {
		fmt.Errorf("failed to allocate codec context")
	}
	defer decodeCtx.AvcodecClose()

	decodeCodecParams := libavcodec.AvcodecParametersAlloc()
	if decodeCodecParams == nil {
		fmt.Errorf("failed to allocate codec parameters")
	}
	decodeCodecParams.CodecType = libavutil.AVMEDIA_TYPE_VIDEO
	decodeCodecParams.CodecId = libavcodec.AV_CODEC_ID_H264
	decodeCodecParams.Format = libavutil.AV_PIX_FMT_YUV420P
	decodeCtx.AvcodecParametersToContext(decodeCodecParams)
	libavcodec.AvcodecParametersFree(&decodeCodecParams)

	frame := libavutil.AvFrameAlloc()
	if frame == nil {
		fmt.Errorf("failed to allocate frame")
	}
	defer libavutil.AvFrameFree(&frame)

	// Encode the scaled frame into a new packet
	encoder := libavcodec.AvcodecFindEncoder(libavcodec.AV_CODEC_ID_H264)
	if encoder == nil {
		fmt.Errorf("failed to find encoder")
	}
	encoderCtx := encoder.AvcodecAllocContext3()
	if encoderCtx == nil {
		fmt.Errorf("failed to allocate encoder context")
	}
	defer encoderCtx.AvcodecClose()

	// ffmpeg编译加上x264
	// FIXME: 找到下面 setParams 对应的函数
	// encoderCtx.SetEncodeParams2(width, height, (libavcodec.PixelFormat)(libavcodec.AV_PIX_FMT_YUV420P), false, 10)

	encoderCtx.Width = int32(width)
	encoderCtx.Height = int32(height)
	// encoderCtx.BitRate = int64(fc.bitrate)
	encoderCtx.GopSize = 10
	encoderCtx.PixFmt = int32(libavutil.AV_PIX_FMT_YUV420P)
	encoderCtx.TimeBase.Num = 1
	encoderCtx.TimeBase.Den = 29

	// defer libswscale.SwsFreeContext(swsCtx)
	scaledFrame := libavutil.AvFrameAlloc()
	if scaledFrame == nil {
		fmt.Errorf("failed to allocate scaled frame")
	}
	defer libavutil.AvFrameFree(&scaledFrame)

	return &FrameConverter{targetWidth: width, targetHeight: height, targetBitrate: bitrate,
		decoder: decoder, decodeCtx: decodeCtx, origFrame: frame,
		scaledFrame: scaledFrame, encoder: encoder, encoderCtx: encoderCtx}
}

// PointerToBytes 函数接收一个指向内存地址的指针和一个长度参数，将指针指向的内存地址中的数据转换为字节切片并返回
func PointerToBytes(pointer unsafe.Pointer, size int) []byte {
	// 将指针转换为一个 byte 类型的指针
	byteArrayPointer := (*[1 << 30]byte)(pointer)
	// 从 byte 数组中获取对应长度的数据
	bytes := byteArrayPointer[:size]
	return bytes
}

// 进行帧数据转换
func (fc FrameConverter) convertFrame(frameData []byte) ([]byte, error) {
	// 打开解码器（部分参数要到这一步才能获取到）
	if fc.decodeCtx.AvcodecIsOpen() <= 0 {
		fc.decodeCtx.Width = int32(fc.originWidth)
		fc.decodeCtx.Height = int32(fc.originHeight)
		if fc.decodeCtx.AvcodecOpen2(fc.decoder, nil) < 0 {
			fmt.Errorf("failed to open codec")
		}
	}

	// encoderCtx.SetBitRate(int64(bitrate))
	if fc.encoderCtx.AvcodecIsOpen() <= 0 {
		// fixme: 删掉这个强制把编码器大小设定为和原始帧大小一致的设定
		fc.encoderCtx.Width = int32(fc.originWidth)
		fc.encoderCtx.Height = int32(fc.originHeight)
		if fc.encoderCtx.AvcodecOpen2(fc.encoder, nil) < 0 {
			fmt.Errorf("failed to open encoder")
		}
	}

	// Decode the frame data into the AVFrame
	packet := libavcodec.AvPacketAlloc()
	packet.AvInitPacket()

	if packet == nil {
		return nil, fmt.Errorf("failed to allocate packet")
	}
	// release after usage
	defer packet.AvFreePacket()

	packet.Data = (*uint8)(unsafe.Pointer(&frameData[0]))
	packet.Size = uint32(len(frameData))
	// data := PointerToBytes(unsafe.Pointer(packet.Data()), int(packet.Size()))
	// print(data)

	// if fc.codecCtx.AvcodecOpen2(&fc.codec, nil) < 0 {
	// 	fmt.Errorf("failed to open codec")
	// }

	// 解码
	response := fc.decodeCtx.AvcodecSendPacket(packet)
	if response < 0 {
		return nil, fmt.Errorf("failed to send packet")
	}

	response = fc.decodeCtx.AvcodecReceiveFrame((*libavcodec.AVFrame)(unsafe.Pointer(fc.origFrame)))
	if response < 0 {
		return nil, fmt.Errorf("failed to receive frame")
	}

	// 初始化缩放
	if fc.swsCtx == nil {
		privateSwsCtx := libswscale.SwsGetContext(
			fc.decodeCtx.Width, fc.decodeCtx.Height, fc.origFrame.Format,
			fc.encoderCtx.Width, fc.encoderCtx.Height, fc.encoderCtx.PixFmt,
			libswscale.SWS_BILINEAR, nil, nil, nil,
		)
		if privateSwsCtx == nil {
			log.Fatalf("failed to create swscale context")
		}
		fc.swsCtx = privateSwsCtx
	}

	// 为缩放后的新 frame 分配空间
	// fixme: 后面必须挪到初始化位置，否则会重复分配，导致内存暴涨
	response = libavutil.AvImageAlloc((*[4]*byte)(unsafe.Pointer(&fc.scaledFrame.Data[0])),
		(*[4]int32)(unsafe.Pointer(&fc.scaledFrame.Linesize[0])),
		fc.encoderCtx.Width, fc.encoderCtx.Height, fc.encoderCtx.PixFmt, 16)
	if response < 0 {
		fmt.Println("Could not allocate target image")
	}

	// libavutil.frame(fc.scaledFrame, fc.width, fc.height, libavutil.AV_PIX_FMT_YUV420P9)
	// fc.scaledFrame.setWidth(width)
	// fc.scaledFrame.SetHeight(height)
	// fc.scaledFrame.SetFormat(int32(libavcodec.AV_PIX_FMT_YUV420P9))
	// if libavutil.AvFrameGetBuffer(fc.scaledFrame, 32) < 0 {
	// 	return nil, fmt.Errorf("failed to allocate buffer for scaled frame")
	// }

	fc.swsCtx.SwsScale(
		(**byte)(unsafe.Pointer(&fc.origFrame.Data)),
		(*int32)(unsafe.Pointer(&fc.origFrame.Linesize)),
		0, uint32(fc.decodeCtx.Height),
		(**byte)(unsafe.Pointer(&fc.scaledFrame.Data)),
		(*int32)(unsafe.Pointer(&fc.scaledFrame.Linesize)),
	)

	encodedPacket := libavcodec.AvPacketAlloc()
	if encodedPacket == nil {
		return nil, fmt.Errorf("failed to allocate packet")
	}
	defer encodedPacket.AvFreePacket()

	// 重新编码
	// // fixme: 手动设定 pts
	// fc.frame.Pts = 0
	if fc.encoderCtx.AvcodecSendFrame((*libavcodec.AVFrame)(unsafe.Pointer(fc.origFrame))) < 0 {
		return nil, fmt.Errorf("failed to send frame to encoder")
	}
	response = fc.encoderCtx.AvcodecReceivePacket(encodedPacket)
	if response < 0 {
		return nil, fmt.Errorf("failed to receive packet from encoder")
	}

	// Return the encoded packet data
	return (*[1 << 30]byte)(unsafe.Pointer(encodedPacket.Data))[:encodedPacket.Size:encodedPacket.Size], nil
}
